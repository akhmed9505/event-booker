package renderer

import (
	"net/http"

	"github.com/wb-go/wbf/ginext"
)

type RenderHandler interface {
	LoginPage(w http.ResponseWriter)
	RegisterPage(w http.ResponseWriter)
}

type Handler struct {
	render RenderHandler
}

func NewHandler(render RenderHandler) *Handler {
	return &Handler{
		render: render,
	}
}

func (h *Handler) Loginpage(c *ginext.Context) {
	h.render.LoginPage(c.Writer)
}

func (h *Handler) Registerpage(c *ginext.Context) {
	h.render.RegisterPage(c.Writer)
}

/*
func (a *Handler) RootRedirect(c *ginext.Context) {
	userInfo, exists := c.Get("userInfo")
	if !exists || userInfo == nil {
		// user not authorized — redirect to /register
		c.Redirect(http.StatusFound, "/register")
		return
	}
	// user authorized — redirect to home
	c.Redirect(http.StatusFound, "/home")
}
*/
