package server

import (
	"context"
	"strings"

	"google.golang.org/grpc"
)

// concurrencyLimiter хранит два семафора: для file-операций и для просмотра списка.
type concurrencyLimiter struct {
	fileOps chan struct{} // семафор для скачивания и загрузки файлов
	listOps chan struct{} // семафор для просмотра списка файлов
}

// newConcurrencyLimiter создаёт новый лимитер с заданными лимитами.
func newConcurrencyLimiter(fileOpsLimit, listOpsLimit int) *concurrencyLimiter {
	return &concurrencyLimiter{
		fileOps: make(chan struct{}, fileOpsLimit),
		listOps: make(chan struct{}, listOpsLimit),
	}
}

// acquire пытается захватить слот в зависимости от метода.
func (cl *concurrencyLimiter) acquire(method string) (release func()) {
	if strings.HasSuffix(method, "/ListFiles") {
		cl.listOps <- struct{}{}
		return func() { <-cl.listOps }
	}
	cl.fileOps <- struct{}{}
	return func() { <-cl.fileOps }
}

// unaryInterceptor для униарных вызовов.
func (cl *concurrencyLimiter) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {

	release := cl.acquire(info.FullMethod)
	defer release()
	return handler(ctx, req)
}

// streamInterceptor для стримовых вызовов.
func (cl *concurrencyLimiter) streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	
	release := cl.acquire(info.FullMethod)
	defer release()
	return handler(srv, ss)
}
