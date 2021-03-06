package service

import (
	"errors"
	"fmt"

	"github.com/goushuyun/taobao-erp/misc"

	"github.com/goushuyun/taobao-erp/db"
	"github.com/goushuyun/taobao-erp/errs"
	"github.com/goushuyun/taobao-erp/pb"
	users_db "github.com/goushuyun/taobao-erp/users/db"
	"golang.org/x/net/context"

	"github.com/goushuyun/taobao-erp/misc/token"
	"github.com/goushuyun/taobao-erp/users/role"
	"github.com/wothing/log"
)

type UsersServer struct {
}

func (s *UsersServer) UserExist(ctx context.Context, req *pb.User) (*pb.UserResp, error) {
	tid := misc.GetTidFromContext(ctx)
	defer log.TraceOut(log.TraceIn(tid, "Login", "%#v", req))

	req.Password = misc.Md5String(req.Password)
	isExist, err := users_db.UserExist(req)
	if err != nil {
		log.Error(err)
		return nil, errs.Wrap(errors.New(err.Error()))
	}

	var msg string
	if isExist {
		msg = "exist"
	} else {
		msg = "not_exist"
	}

	return &pb.UserResp{Code: errs.Ok, Message: msg}, nil
}

func (s *UsersServer) Login(ctx context.Context, req *pb.User) (*pb.UserResp, error) {
	tid := misc.GetTidFromContext(ctx)
	defer log.TraceOut(log.TraceIn(tid, "Login", "%#v", req))

	req.Password = misc.Md5String(req.Password)
	err := users_db.Login(req)

	if err != nil {
		// not found this user
		if err.Error() == "not_found" {
			return &pb.UserResp{Code: errs.Ok, Message: "not_found"}, nil
		} else {
			// met other error
			log.Error(err)
			return nil, errs.Wrap(errors.New(err.Error()))
		}
	}

	// has this user, sign token
	token := token.SignUserToken(token.InterToken, req.Id, req.Mobile, req.Name, req.Role)
	req.Password = "****"
	return &pb.UserResp{Code: errs.Ok, Message: "ok", Token: token, Data: req}, nil
}

func (s *UsersServer) Register(ctx context.Context, req *pb.User) (*pb.UserResp, error) {
	tid := misc.GetTidFromContext(ctx)
	defer log.TraceOut(log.TraceIn(tid, "Register", "%#v", req))

	// check if identifying code is ok
	conn := db.GetRedisConn()
	defer conn.Close()

	checkcode, err := conn.Do("get", req.Mobile)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// checkcode is expired
	if checkcode == nil {
		return &pb.UserResp{Code: errs.Ok, Message: "checkcode_expired"}, nil
	}

	// checkcode is error
	checkcode = fmt.Sprintf("%s", checkcode)
	if checkcode != req.Checkcode {
		return &pb.UserResp{Code: errs.Ok, Message: "checkcode_error"}, nil
	}

	// insert user to db
	req.Password = misc.Md5String(req.Password) //encry Password using md5
	err = users_db.SaveUser(req)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// sign token
	token := token.SignUserToken(token.InterToken, req.Id, req.Mobile, req.Name, role.InterNormalUser)
	req.Password = "*****"

	return &pb.UserResp{Code: errs.Ok, Message: "ok", Data: req, Token: token}, nil
}
