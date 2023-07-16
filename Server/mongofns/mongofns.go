package mongofns

import (
	// "go.mongodb.org/mongo-driver/bson"
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// connect is a userdefined function returns mongo.Client,
// context.Context, context.CancelFunc and error.
// context.Context is used to set a deadline for process
// context.CancelFunc is used to cancel context and its resources

func Connect(uri string) (*mongo.Client, context.Context,
	context.CancelFunc, error) {

	ctx, cancel := context.WithCancel(context.Background())
	// ctx, cancel := context.WithTimeout(context.Background())
	// 30*time.Second)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	return client, ctx, cancel, err
}

// close is a userdef func to close resources
// closes MongoDB connection and cancel context

func Close(client *mongo.Client, ctx context.Context,
	cancel context.CancelFunc) {

	defer cancel()
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()
}
