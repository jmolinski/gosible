package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/scylladb/gosible/modules"
	pb "github.com/scylladb/gosible/remote/proto"
	"github.com/scylladb/gosible/utils/stdIoConn"
	"github.com/scylladb/gosible/utils/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"os"
)

type server struct {
	pb.UnimplementedGosibleClientServer

	modules *modules.ModuleRegistry
}

func (s *server) ExecuteModule(_ context.Context, req *pb.ExecuteModuleRequest) (*pb.ExecuteModuleReply, error) {
	action, ok := s.modules.FindModule(req.ModuleName)
	if !ok {
		return nil, errors.New("module not found")
	}

	vars := types.Vars{}
	if err := json.Unmarshal(req.VarsJson, &vars); err != nil {
		return nil, err
	}
	ctx := &modules.RunContext{
		MetaArgs: req.MetaArgs,
	}
	result := action.Run(ctx, vars)
	resultJson, err := json.Marshal(&result)
	if err != nil {
		return nil, err
	}
	return &pb.ExecuteModuleReply{
		ReturnValueJson: resultJson,
	}, nil
}

func setupRpcServer(modules *modules.ModuleRegistry) {
	grpcS := grpc.NewServer()
	conn := stdIoConn.NewFileConn(os.Stdin, os.Stdout, grpcS.GracefulStop)
	listener := stdIoConn.NewPreconnectedListener(conn)
	s := newServer(modules)

	registerServer(grpcS, s)
	reflection.Register(grpcS)

	if err := grpcS.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func newServer(modules *modules.ModuleRegistry) *server {
	return &server{
		modules: modules,
	}
}

func registerServer(grpcS *grpc.Server, s *server) {
	pb.RegisterGosibleClientServer(grpcS, s)
}
