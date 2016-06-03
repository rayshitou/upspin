// Copyright 2016 The Upspin Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package remote implements an inprocess store server that uses RPC to
// connect to a remote store server.
package remote

import (
	"errors"

	gContext "golang.org/x/net/context"

	"upspin.io/auth/grpcauth"
	"upspin.io/bind"
	"upspin.io/upspin"
	"upspin.io/upspin/proto"
)

// dialContext contains the destination and authenticated user of the dial.
type dialContext struct {
	endpoint upspin.Endpoint
	userName upspin.UserName
}

// remote implements upspin.Store.
type remote struct {
	grpcauth.AuthClientService // For handling Authenticate, Ping and Close.
	ctx                        dialContext
	storeClient                proto.StoreClient
}

var _ upspin.Store = (*remote)(nil)

// Get implements upspin.Store.Get.
func (r *remote) Get(ref upspin.Reference) ([]byte, []upspin.Location, error) {
	gCtx, err := r.SetAuthContext(r.Context)
	if err != nil {
		return nil, nil, err
	}
	req := &proto.StoreGetRequest{
		Reference: string(ref),
	}
	resp, err := r.storeClient.Get(gCtx, req)
	return resp.Data, proto.UpspinLocations(resp.Locations), err
}

// Put implements upspin.Store.Put.
// Directories are created with MakeDirectory.
func (r *remote) Put(data []byte) (upspin.Reference, error) {
	gCtx, err := r.SetAuthContext(r.Context)
	if err != nil {
		return "", err
	}
	req := &proto.StorePutRequest{
		Data: data,
	}
	resp, err := r.storeClient.Put(gCtx, req)
	if err != nil {
		return "", err
	}
	return upspin.Reference(resp.Reference), nil
}

// Delete implements upspin.Store.Delete.
func (r *remote) Delete(ref upspin.Reference) error {
	gCtx, err := r.SetAuthContext(r.Context)
	if err != nil {
		return err
	}
	req := &proto.StoreDeleteRequest{
		Reference: string(ref),
	}
	_, err = r.storeClient.Delete(gCtx, req)
	return err
}

// ServerUserName implements upspin.Service.
func (r *remote) ServerUserName() string {
	return "" // No one is authenticated.
}

// Endpoint implements upspin.Store.Endpoint.
func (r *remote) Endpoint() upspin.Endpoint {
	return r.ctx.endpoint
}

// Configure implements upspin.Service.
func (r *remote) Configure(options ...string) error {
	req := &proto.ConfigureRequest{
		Options: options,
	}
	_, err := r.storeClient.Configure(gContext.Background(), req)
	return err
}

// Dial implements upspin.Service.
func (*remote) Dial(context *upspin.Context, e upspin.Endpoint) (upspin.Service, error) {
	if e.Transport != upspin.Remote {
		return nil, errors.New("remote: unrecognized transport")
	}

	conn, err := grpcauth.NewGRPCClient(e.NetAddr, grpcauth.AllowSelfSignedCertificate)
	if err != nil {
		return nil, err
	}
	// TODO: When can we do conn.Close()?
	storeClient := proto.NewStoreClient(conn)
	authClient := grpcauth.AuthClientService{
		GRPCCommon: storeClient,
		GRPCConn:   conn,
		Context:    context,
	}
	r := &remote{
		AuthClientService: authClient,
		ctx: dialContext{
			endpoint: e,
			userName: context.UserName,
		},
		storeClient: storeClient,
	}

	return r, nil
}

const transport = upspin.Remote

func init() {
	r := &remote{} // uninitialized until Dial time.
	bind.RegisterStore(transport, r)
}