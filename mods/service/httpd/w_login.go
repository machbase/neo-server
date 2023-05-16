package httpd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/machbase/neo-server/mods/service/security"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

func (svr *httpd) issueAccessToken(loginName string) (accessToken string, refreshToken string, refreshTokenId string, err error) {
	claim := security.NewClaim(loginName)
	accessToken, err = security.SignTokenWithClaim(claim)
	if err != nil {
		err = errors.Wrap(err, "signing at error")
		return
	}

	refreshClaim := security.NewClaimForRefresh(claim)
	refreshToken, err = security.SignTokenWithClaim(refreshClaim)
	if err != nil {
		err = errors.Wrap(err, "signing rt error")
		return
	}
	refreshTokenId = refreshClaim.ID
	return
}

func (svr *httpd) verifyAccessToken(token string) (security.Claim, error) {
	claim := security.NewClaimEmpty()
	ok, err := security.VerifyTokenWithClaim(token, claim)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return claim, nil
}

func IsErrTokenExpired(err error) bool {
	if jwtErr, ok := err.(*jwt.ValidationError); ok && jwtErr.Is(jwt.ErrTokenExpired) {
		return true
	}
	return false
}

type LoginReq struct {
	LoginName string `json:"loginName"`
	Password  string `json:"password"`
}

type LoginRsp struct {
	Success      bool   `json:"success"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Reason       string `json:"reason"`
	Elapse       string `json:"elapse"`
}

func (svr *httpd) handleLogin(ctx *gin.Context) {
	var req = &LoginReq{}
	var rsp = &LoginRsp{
		Success: false,
		Reason:  "not specified",
	}

	tick := time.Now()

	err := ctx.Bind(req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if len(req.LoginName) == 0 {
		rsp.Reason = "missing required loginName field"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	passed := false

	if dbauth, ok := svr.db.(spi.DatabaseAuth); !ok {
		svr.log.Warnf("database auth is not implemented by %T", svr.db)
		rsp.Reason = fmt.Sprintf("database (%T) is not supporting user authentication", svr.db)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		passed, err = dbauth.UserAuth(req.LoginName, req.Password)
		if err != nil {
			svr.log.Warnf("database auth failed %s", err.Error())
			rsp.Reason = "database error for user authentication"
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	if !passed {
		svr.log.Tracef("'%s' login fail password mis-matched", req.LoginName)
		rsp.Reason = "user not found or invalid password"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	accessToken, refreshToken, refreshTokenId, err := svr.issueAccessToken(req.LoginName)
	svr.log.Tracef("'%s' login success %s", req.LoginName, refreshTokenId)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	// cache username and password for web-terminal uses
	svr.neoShellAccount[req.LoginName] = req.Password

	// store refresh token
	svr.jwtCache.SetRefreshToken(refreshTokenId, refreshToken)

	rsp.Success = true
	rsp.Reason = "success"
	rsp.AccessToken = accessToken
	rsp.RefreshToken = refreshToken
	rsp.Elapse = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
}

type ReLoginReq struct {
	RefreshToken string `json:"refreshToken"`
}

type ReLoginRsp LoginRsp

func (svr *httpd) handleReLogin(ctx *gin.Context) {
	var req ReLoginReq
	var rsp = &ReLoginRsp{
		Success: false,
		Reason:  "not specified",
	}

	tick := time.Now()

	err := ctx.Bind(&req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// request로 전달받은 refresh token의 refreshClaim으로 변환하면서 verified 확인한다.
	refreshClaim := security.NewClaimEmpty()
	verified, err := security.VerifyTokenWithClaim(req.RefreshToken, refreshClaim)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	if !verified {
		rsp.Reason = "not verified refresh token"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}

	svr.log.Tracef("'%s' relogin", refreshClaim.Subject)

	// 저장되어 있는 refresh token과 비교한다.
	// load refresh token from cached table by claim.ID
	storedToken, ok := svr.jwtCache.GetRefreshToken(refreshClaim.ID)
	if !ok {
		rsp.Reason = "refresh token not found"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	if req.RefreshToken != storedToken {
		rsp.Reason = "invalid refresh token"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}

	// 저장되어 있던 refresh token과 요청한 refresh token이 일치하면
	// access token을 재발급한다.

	/// Note:
	///   refreshToken으로 새로운 accessToken을 갱신하는 과정에서
	///   refreshToken 자체는 갱신하거나/갱신하지 않는 두 가지 선택이 있는데
	///     1) 여기처럼 갱신하는 정책은 사용자가 지속적으로 시스템을 사용하는 경우 ID/PW로 로그인을 다시하지 않아도 된다.
	///     2) 갱신하지 않는 경우는 refreshToken의 expire 주기마다 로그인을 수행해야 한다.
	accessToken, refreshToken, refreshTokenId, err := svr.issueAccessToken(refreshClaim.Subject)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	// 신규 발급된 refresh token을 저장한다.
	svr.jwtCache.SetRefreshToken(refreshTokenId, refreshToken)

	rsp.Success, rsp.Reason = true, "success"
	rsp.AccessToken = accessToken
	rsp.RefreshToken = refreshToken
	rsp.Elapse = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
}

type LogoutReq struct {
	RefreshToken string `json:"refreshToken"`
}

type LogoutRsp struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (svr *httpd) handleLogout(ctx *gin.Context) {
	tick := time.Now()

	var req = &LogoutReq{}
	var rsp = &LogoutRsp{
		Success: false,
		Reason:  "not specified",
	}
	err := ctx.Bind(req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	refreshClaim := security.NewClaimEmpty()
	_, err = security.VerifyTokenWithClaim(req.RefreshToken, refreshClaim)
	if err == nil && len(refreshClaim.ID) > 0 {
		// delete stored refresh token by claim.ID
		svr.jwtCache.RemoveRefreshToken(refreshClaim.ID)
	}

	svr.log.Tracef("logout '%s' success rt.id:'%s'", refreshClaim.Subject, refreshClaim.ID)

	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleCheck(ctx *gin.Context) {
	if o := ctx.Value("jwt-claim"); o != nil {
		if claim, ok := o.(security.Claim); ok {
			if err := claim.Valid(); err == nil {
				ctx.JSON(http.StatusNoContent, "")
			}
		}
	}
	ctx.JSON(http.StatusUnauthorized, "")
}
