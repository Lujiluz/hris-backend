package redis

import (
	"context"
	"fmt"
	"time"

	"hris-backend/internal/domain"

	redisClient "github.com/redis/go-redis/v9"
)

type otpRepo struct {
	redis *redisClient.Client
}

func NewOTPRepository(redis *redisClient.Client) domain.OTPRepository {
	return &otpRepo{redis: redis}
}

func (r *otpRepo) SetOTP(ctx context.Context, email string, otp string, expiration time.Duration) error {
	key := fmt.Sprintf("otp:%s", email)
	return r.redis.Set(ctx, key, otp, expiration).Err()
}

func (r *otpRepo) GetOTP(ctx context.Context, email string) (string, error) {
	key := fmt.Sprintf("otp:%s", email)
	return r.redis.Get(ctx, key).Result()
}

func (r *otpRepo) DeleteOTP(ctx context.Context, email string) error {
	key := fmt.Sprintf("otp:%s", email)
	return r.redis.Del(ctx, key).Err()
}
