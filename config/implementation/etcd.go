package implementation

import (
	"context"
	"fmt"
	"github.com/no-mole/neptune/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var RegistryImplementationTypeNameEtcd = "etcd"

func init() {
	config.RegistryImplementation(RegistryImplementationTypeNameEtcd, func(ctx context.Context) config.Client {
		return &EtcdConfigClient{}
	})
}

var (
	_ config.Client = &EtcdConfigClient{} //ensure EtcdConfigClient Implementation Client

)

type EtcdConfigClient struct {
	config  *config.Config
	client  *clientv3.Client
	closeCh chan struct{}
}

func (s *EtcdConfigClient) Init(ctx context.Context, conf *config.Config) error {
	s.config = conf

	clientConf := config.Trans2EtcdConfig(ctx, conf)

	cli, err := clientv3.New(clientConf)
	if err != nil {
		return err
	}
	s.client = cli
	return nil
}

func (s *EtcdConfigClient) Close() error {
	close(s.closeCh)
	return s.client.Close()
}

func (s *EtcdConfigClient) Set(ctx context.Context, key, value string) error {
	_, err := s.client.Put(ctx, s.genKey(key), value)
	return err
}

func (s *EtcdConfigClient) Get(ctx context.Context, key string) (*config.Item, error) {
	resp, err := s.client.Get(ctx, s.genKey(key))
	if err != nil {
		return nil, err
	}
	value := ""
	if len(resp.Kvs) == 0 {
		value = ""
	} else {
		value = string(resp.Kvs[0].Value)
	}
	return config.NewItem(s.config.Namespace, key, value), nil
}

func (s *EtcdConfigClient) Exist(ctx context.Context, key string) (bool, error) {
	resp, err := s.client.Get(ctx, s.genKey(key))
	if err != nil {
		return false, err
	}
	return len(resp.Kvs) != 0, nil
}

func (s *EtcdConfigClient) Watch(ctx context.Context, item *config.Item, callback func(item *config.Item)) error {
	if callback == nil {
		return nil
	}
	watchCh := s.client.Watch(ctx, s.genKey(item.Key))
	go func() {
		for {
			select {
			case wResp := <-watchCh:
				if len(wResp.Events) > 0 {
					event := wResp.Events[len(wResp.Events)-1]
					item.SetValue(string(event.Kv.Value))
					callback(item)
				}
			case <-s.closeCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *EtcdConfigClient) genKey(key string) string {
	return fmt.Sprintf("/%s/%s", s.config.Namespace, key)
}
