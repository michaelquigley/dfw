package pdf

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/michaelquigley/df/dd"
)

const maxMessage = 8 << 20

// dd binds structs; the extra field preserves the foreign protocol's exact keys.
type wireMessage struct {
	Fields map[string]any `dd:",+extra"`
}

type reply struct {
	value map[string]any
	err   error
}

// pipe correlates replies while keeping events independent of a waiting caller.
// closing either side releases all callers, including a blocked pipe writer.
type pipe struct {
	reader  io.ReadCloser
	writer  io.WriteCloser
	mu      sync.Mutex
	next    uint64
	pending map[uint64]chan reply
	err     error
	done    chan struct{}
	out     chan []byte
	events  chan map[string]any
	workers sync.WaitGroup
}

func newPipe(reader io.ReadCloser, writer io.WriteCloser) *pipe {
	p := &pipe{reader: reader, writer: writer, pending: make(map[uint64]chan reply), done: make(chan struct{}), out: make(chan []byte, 64), events: make(chan map[string]any, 256)}
	p.workers.Add(2)
	go p.readLoop()
	go p.writeLoop()
	return p
}

func (p *pipe) fail(err error) {
	p.mu.Lock()
	if p.err != nil {
		p.mu.Unlock()
		return
	}
	p.err = err
	close(p.done)
	for id, ch := range p.pending {
		ch <- reply{err: err}
		delete(p.pending, id)
	}
	p.mu.Unlock()
	_ = p.reader.Close()
	_ = p.writer.Close()
}

func (p *pipe) close() {
	p.fail(Failure(RenderFailed, "browser connection closed"))
	p.workers.Wait()
}

func (p *pipe) call(ctx context.Context, method string, params map[string]any, session string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.err != nil {
		err := p.err
		p.mu.Unlock()
		return nil, err
	}
	p.next++
	id := p.next
	ch := make(chan reply, 1)
	p.pending[id] = ch
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.pending, id); p.mu.Unlock() }()
	message := map[string]any{"id": id, "method": method, "params": params}
	if session != "" {
		message["sessionId"] = session
	}
	data, err := dd.UnbindJSON(wireMessage{Fields: message})
	if err != nil {
		return nil, Failure(RenderFailed, "encode browser command")
	}
	data = append(data, 0)
	select {
	case p.out <- data:
	case <-p.done:
		return nil, p.failure()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case r := <-ch:
		return r.value, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *pipe) failure() error { p.mu.Lock(); defer p.mu.Unlock(); return p.err }

func splitMessage(data []byte, atEOF bool) (int, []byte, error) {
	if end := bytes.IndexByte(data, 0); end >= 0 {
		return end + 1, data[:end], nil
	}
	if atEOF && len(data) != 0 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	return 0, nil, nil
}

func (p *pipe) readLoop() {
	defer p.workers.Done()
	scanner := bufio.NewScanner(p.reader)
	scanner.Buffer(make([]byte, 4096), maxMessage)
	scanner.Split(splitMessage)
	for scanner.Scan() {
		var envelope wireMessage
		if err := dd.BindJSONReader(&envelope, bytes.NewReader(scanner.Bytes())); err != nil {
			p.fail(Failure(RenderFailed, "malformed browser message"))
			return
		}
		message := envelope.Fields
		if value, exists := message["id"]; exists {
			n, ok := value.(float64)
			if !ok || n < 1 || n > 1<<53 || n != float64(uint64(n)) {
				p.fail(Failure(RenderFailed, "invalid browser reply id"))
				return
			}
			id := uint64(n)
			p.mu.Lock()
			ch := p.pending[id]
			delete(p.pending, id)
			p.mu.Unlock()
			if ch != nil {
				r := reply{value: object(message, "result")}
				if message["error"] != nil {
					r.err = Failure(Incompatible, "browser rejected a required PDF protocol command")
				}
				ch <- r
			}
		} else if text(message, "method") != "" {
			select {
			case p.events <- message:
			case <-p.done:
				return
			default:
				p.fail(Failure(RenderFailed, "browser event queue exceeded its limit"))
				return
			}
		} else {
			p.fail(Failure(RenderFailed, "invalid browser message"))
			return
		}
	}
	p.fail(Failure(RenderFailed, "browser connection ended or exceeded its message limit"))
}

func (p *pipe) writeLoop() {
	defer p.workers.Done()
	for {
		select {
		case <-p.done:
			return
		case data := <-p.out:
			for len(data) > 0 {
				n, err := p.writer.Write(data)
				if err != nil || n == 0 {
					p.fail(Failure(RenderFailed, "write to browser connection failed"))
					return
				}
				data = data[n:]
			}
		}
	}
}
