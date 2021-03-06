package grpc

import (
	"context"
	"strings"

	"github.com/kintoproj/go-utils/server"
	"google.golang.org/grpc/metadata"
)

func GetAuthBearerTokenFromHeader(ctx context.Context) (string, *server.Error) {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return "", server.NewError(
			server.StatusCode_InternalServerError, "could not parse grpc metadata from grpc context?")
	}

	const grpcAuthorizationHeaderKey = "authorization"
	authorizationArray := md.Get(grpcAuthorizationHeaderKey)
	arrLen := len(authorizationArray)

	// default empty for public requests
	token := ""
	if arrLen > 1 {
		return "", server.NewError(server.StatusCode_BadRequest,
			"invalid authorization metadata - can only have one authorization header!")
	} else if arrLen == 1 {
		const bearerTokenPrefix = "Bearer "
		token = strings.TrimPrefix(authorizationArray[0], bearerTokenPrefix)
	}

	return token, nil
}
