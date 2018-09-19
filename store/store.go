package store

import "github.com/gomodule/redigo/redis"

const (
	NameserversKey = "io.boss.nameservers"
)

func New(pool *redis.Pool) *Store {
	return &Store{
		pool: pool,
	}
}

type Store struct {
	pool *redis.Pool
}

func (s *Store) do(action string, args ...interface{}) (interface{}, error) {
	conn := s.pool.Get()
	defer conn.Close()
	return conn.Do(action, args...)
}
