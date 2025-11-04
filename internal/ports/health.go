package ports

import "context"

type HealthPort interface {
	Check(ctx context.Context, name string) (healthy bool, msg string)
}
