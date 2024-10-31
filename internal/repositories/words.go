package repositories

import (
	"context"
	"time"

	"github.com/Calgorr/EnglishPinglish/config"
	"github.com/redis/go-redis/v9"
)

type WordsRepository interface {
	GetWord(context.Context, string) (string, error)
	SetWord(context.Context, string, string, time.Duration) error
}

type wordsRepositoryImpl struct {
	redisClient *redis.Client
}

func NewWordsRepository(redisConfig config.RedisConfig) WordsRepository {
	client := redis.NewClient(&redis.Options{
		Addr: redisConfig.Addr,
	})
	return &wordsRepositoryImpl{
		redisClient: client,
	}
}
func (r *wordsRepositoryImpl) GetWord(ctx context.Context, key string) (string, error) {
	return r.redisClient.Get(ctx, key).Result()
}

func (r *wordsRepositoryImpl) SetWord(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.redisClient.Set(ctx, key, value, ttl).Err()
}
