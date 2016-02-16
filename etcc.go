package etcc

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
	"golang.org/x/net/context"
)

type Etcd struct {
	api client.KeysAPI
}

// New trys to connect to etcd server.  endpoints must be addreses
// delimited by comma, like "http://127.0.0.1:4001,http://127.0.0.1:2379".
func New(endpoints string) (*Etcd, error) {
	eps := strings.Split(endpoints, ",")
	for i, ep := range eps {
		u, e := url.Parse(ep)
		if e != nil {
			return nil, fmt.Errorf("url.Parse: %v", e)
		}

		if u.Scheme == "" {
			u.Scheme = "http"
		}
		eps[i] = u.String()
	}

	tr, e := transport.NewTransport(transport.TLSInfo{}, 10*time.Second)
	if e != nil {
		return nil, e
	}

	c, e := client.New(client.Config{Endpoints: eps, Transport: tr})
	if e != nil {
		return nil, e
	}

	ctx, cancel := timeoutContext()
	e = c.Sync(ctx)
	cancel()
	if e != nil {
		return nil, e
	}

	return &Etcd{client.NewKeysAPI(c)}, nil
}

func timeoutContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), client.DefaultRequestTimeout)
}

// Mkdir creates a directory. The directory could be multiple-level,
// like /home/yi/hello. But it must not exist before; otherwise Mkdir
// returns an error.
func (c *Etcd) Mkdir(dir string) error {
	ctx, cancel := timeoutContext()
	defer cancel()
	_, e := c.api.Set(ctx, dir, "", &client.SetOptions{Dir: true, PrevExist: client.PrevNoExist})
	return e
}

func (c *Etcd) SetWithTTL(key, value string, ttl time.Duration) error {
	ctx, cancel := timeoutContext()
	defer cancel()
	_, e := c.api.Set(ctx, key, value, &client.SetOptions{TTL: ttl})
	return e
}

func (c *Etcd) Set(key, value string) error {
	return c.SetWithTTL(key, value, time.Duration(0))
}

func (c *Etcd) Get(key string) (string, error) {
	ctx, cancel := timeoutContext()
	defer cancel()
	r, e := c.api.Get(ctx, key, &client.GetOptions{Sort: true})
	if e != nil {
		return "", e
	}
	return r.Node.Value, nil
}

// Rm removes a either a key-value pair or a directory.  If it is a
// directory, Rm removes all recursive content as well.
func (c *Etcd) Rm(key string) error {
	ctx, cancel := timeoutContext()
	defer cancel()
	_, e := c.api.Delete(ctx, key, &client.DeleteOptions{Recursive: true})
	return e
}

func (c *Etcd) Ls(key string) ([]string, error) {
	if len(key) == 0 {
		key = "/"
	}

	ctx, cancel := timeoutContext()
	defer cancel()

	resp, e := c.api.Get(ctx, key, &client.GetOptions{Sort: false, Recursive: false, Quorum: true})

	if e != nil {
		return nil, e
	}

	if !resp.Node.Dir {
		return []string{resp.Node.Key}, nil
	}

	var keys []string
	for _, node := range resp.Node.Nodes {
		keys = append(keys, node.Key)
	}
	return keys, e
}