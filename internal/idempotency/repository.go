package idempotency

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrKeyNotFound = errors.New("idempotency: key not found")

type IdempotencyRepository interface {
	AcquireLock(ctx context.Context, lockKey string, ownerID string, ttl time.Duration) (bool, error)
	SetPending(ctx context.Context, dataKey string, ttl time.Duration) error
	SaveResponse(ctx context.Context, lockKey string, dataKey string, ownerID string, payload []byte, ttl time.Duration) error
	GetResponse(ctx context.Context, dataKey string) ([]byte, error)
	DeleteLock(ctx context.Context, lockKey string, ownerID string) error
	ForceDelete(ctx context.Context, key string) error
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

// lockKey  "idem:lock:{key}"  TTL corto  (60s)  — quién tiene el candado
// dataKey  "idem:data:{key}"  TTL largo  (24h)  — estado / payload

// AcquireLock hace SET NX en lockKey con ownerID como valor.
// Retorna true si se adquirió el lock.
func (r *RedisStore) AcquireLock(ctx context.Context, lockKey, ownerID string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, lockKey, ownerID, ttl).Result()
}

// SetPending escribe el estado PENDING en dataKey.
func (r *RedisStore) SetPending(ctx context.Context, dataKey string, ttl time.Duration) error {
	return r.client.Set(ctx, dataKey, string(Pending), ttl).Err()
}

// saveResponseScript verifica ownership antes de guardar el resultado.
// Si lockKey ya no pertenece a ownerID (expiró y otro lo tomó), no escribe.
// KEYS[1] = lockKey, KEYS[2] = dataKey
// ARGV[1] = ownerID, ARGV[2] = payload, ARGV[3] = ttl en milisegundos
var saveResponseScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
		return 1
	end
	return 0
`)

// SaveResponse guarda el payload en dataKey solo si aún somos dueños del lock.
func (r *RedisStore) SaveResponse(ctx context.Context, lockKey, dataKey, ownerID string, payload []byte, ttl time.Duration) error {
	ttlMs := ttl.Milliseconds()
	result, err := saveResponseScript.Run(ctx, r.client, []string{lockKey, dataKey}, ownerID, payload, ttlMs).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		// El lock ya no nos pertenece — otro request tomó ownership
		return errors.New("idempotency: lock ownership lost before SaveResponse")
	}
	return nil
}

// GetResponse lee el estado o payload desde dataKey.
// Retorna ErrKeyNotFound si la key no existe o expiró.
func (r *RedisStore) GetResponse(ctx context.Context, dataKey string) ([]byte, error) {
	data, err := r.client.Get(ctx, dataKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return data, nil
}

// deleteLockScript verifica ownership antes de borrar el lock.
// Si el lock expiró y otro lo tomó, no borramos el lock ajeno.
// KEYS[1] = lockKey, ARGV[1] = ownerID
var deleteLockScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	end
	return 0
`)

// DeleteLock borra lockKey solo si somos los dueños del lock.
func (r *RedisStore) DeleteLock(ctx context.Context, lockKey, ownerID string) error {
	return deleteLockScript.Run(ctx, r.client, []string{lockKey}, ownerID).Err()
}

func (r *RedisStore) ForceDelete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}
