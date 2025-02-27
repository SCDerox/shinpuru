package controllers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sarulabs/di/v2"
	"github.com/sirupsen/logrus"
	"github.com/zekroTJA/shinpuru/internal/services/database"
	"github.com/zekroTJA/shinpuru/internal/services/webserver/auth"
	"github.com/zekroTJA/shinpuru/internal/services/webserver/v1/models"
	"github.com/zekroTJA/shinpuru/internal/util/static"
	"github.com/zekroTJA/shinpuru/pkg/discordoauth/v2"
)

type AuthController struct {
	discordOAuth *discordoauth.DiscordOAuth
	rth          auth.RefreshTokenHandler
	ath          auth.AccessTokenHandler
	authMw       auth.Middleware
}

func (c *AuthController) Setup(container di.Container, router fiber.Router) {
	c.discordOAuth = container.Get(static.DiDiscordOAuthModule).(*discordoauth.DiscordOAuth)
	c.rth = container.Get(static.DiAuthRefreshTokenHandler).(auth.RefreshTokenHandler)
	c.ath = container.Get(static.DiAuthAccessTokenHandler).(auth.AccessTokenHandler)
	c.authMw = container.Get(static.DiAuthMiddleware).(auth.Middleware)

	router.Get("/login", c.discordOAuth.HandlerInit)
	router.Get("/oauthcallback", c.discordOAuth.HandlerCallback)
	router.Post("/accesstoken", c.postAccessToken)
	router.Get("/check", c.authMw.Handle, c.getCheck)
	router.Post("/logout", c.authMw.Handle, c.postLogout)
}

// @Summary Access Token Exchange
// @Description Exchanges the cookie-passed refresh token with a generated access token.
// @Tags Authorization
// @Accept json
// @Produce json
// @Success 200 {object} models.AccessTokenResponse
// @Failure 401 {object} models.Error
// @Router /auth/accesstoken [post]
func (c *AuthController) postAccessToken(ctx *fiber.Ctx) error {
	refreshToken := ctx.Cookies(static.RefreshTokenCookieName)
	if refreshToken == "" {
		return fiber.ErrUnauthorized
	}

	ident, err := c.rth.ValidateRefreshToken(refreshToken)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		logrus.WithError(err).Error("WEBSERVER :: failed validating refresh token")
	}
	if ident == "" {
		return fiber.ErrUnauthorized
	}

	token, expires, err := c.ath.GetAccessToken(ident)
	if err != nil {
		return err
	}

	return ctx.JSON(&models.AccessTokenResponse{
		Token:   token,
		Expires: expires,
	})
}

// @Summary Authorization Check
// @Description Returns OK if the request is authorized.
// @Tags Authorization
// @Accept json
// @Produce json
// @Success 200 {object} models.Status
// @Failure 401 {object} models.Error
// @Router /auth/check [get]
func (c *AuthController) getCheck(ctx *fiber.Ctx) error {
	return ctx.JSON(models.Ok)
}

// @Summary Logout
// @Description Reovkes the currently used access token and clears the refresh token.
// @Tags Authorization
// @Accept json
// @Produce json
// @Success 200 {object} models.Status
// @Router /auth/logout [post]
func (c *AuthController) postLogout(ctx *fiber.Ctx) error {
	uid := ctx.Locals("uid").(string)

	err := c.rth.RevokeToken(uid)
	if err != nil && !database.IsErrDatabaseNotFound(err) {
		return err
	}

	ctx.ClearCookie(static.RefreshTokenCookieName)

	return ctx.JSON(models.Ok)
}
