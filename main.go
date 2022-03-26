package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"sort"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/apple"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/twitter"
)

func init() {
	gin.ForceConsoleColor()
}

type App struct {
	route *gin.Engine
}

func routing() *gin.Engine {
	r := gin.Default()

	// Routes
	tasks := r.Group("/task")
	{
		tasks.GET("/auth/{provider}/callback")
		tasks.GET("/:task_name/fields")
	}
	return r
}

func NewApp() *App {
	r := routing()
	if r == nil {
		log.Fatalln("r is nil")
	}
	return &App{
		route: r,
	}
}

func (a *App) Start(restPort string) error {
	errChan := make(chan error)
	go func() {
		err := a.route.Run(restPort)
		if err != nil {
			log.Println(err)
			errChan <- err
			return
		}
	}()
	return <-errChan
}

func main() {
	goth.UseProviders(
		twitter.New("key", "secret", "http://localhost:3000/auth/twitter/callback"),
		google.New("key", "secret", "http://localhost:3000/auth/google/callback"),
		github.New("key", "secret", "http://localhost:3000/auth/github/callback"),
		apple.New(os.Getenv("APPLE_KEY"), os.Getenv("APPLE_SECRET"), "http://localhost:3000/auth/apple/callback", nil, apple.ScopeName, apple.ScopeEmail),
	)

	m := make(map[string]string)
	m["github"] = "Github"
	m["google"] = "Google"
	m["twitter"] = "Twitter"
	m["apple"] = "Apple"

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	providerIndex = &ProviderIndex{Providers: keys, ProvidersMap: m}

	r := gin.Default()
	store := cookie.NewStore([]byte("armin"))
	store.Options(sessions.Options{
		MaxAge: int(time.Second * time.Duration(1800)), //30min
		Path:   "/",
	})
	r.Use(sessions.Sessions("dotenx", store))
	r.GET("/auth/:provider/callback", sessions.Sessions("dotenx_session", store), func(c *gin.Context) {
		prov := c.Param("provider")
		q := c.Request.URL.Query()
		q.Add("provider", prov)
		c.Request.URL.RawQuery = q.Encode()
		user, err := gothic.CompleteUserAuth(c.Writer, c.Request)
		if err != nil {
			fmt.Fprintln(c.Writer, err)
			return
		}
		session := sessions.Default(c)
		session.Set("isAdmin", true)
		err = session.Save()
		if err != nil {
			fmt.Fprintln(c.Writer, err)
			return
		}
		t, _ := template.New("foo").Parse(userTemplate)
		t.Execute(c.Writer, user)
	})

	r.GET("/logout/:provider", sessions.Sessions("dotenx_session", store), func(c *gin.Context) {
		session := sessions.Default(c)
		fmt.Println("#######################")
		fmt.Println(session)
		fmt.Println(session.Get("isAdmin"))
		fmt.Println("#######################")
		/*prov := c.Param("provider")
		q := c.Request.URL.Query()
		q.Add("provider", prov)
		c.Request.URL.RawQuery = q.Encode()
		gothic.Logout(c.Writer, c.Request)
		c.Writer.Header().Set("Location", "/")
		c.Writer.WriteHeader(http.StatusTemporaryRedirect)*/
	})

	r.GET("/auth/:provider", sessions.Sessions("dotenx_session", store), func(c *gin.Context) {
		// try to GET the user without re-authenticating
		prov := c.Param("provider")
		q := c.Request.URL.Query()
		q.Add("provider", prov)
		c.Request.URL.RawQuery = q.Encode()
		if gothUser, err := gothic.CompleteUserAuth(c.Writer, c.Request); err == nil {
			session := sessions.Default(c)
			session.Set("isAdmin", true)
			err = session.Save()
			if err != nil {
				fmt.Fprintln(c.Writer, err)
				return
			}
			t, _ := template.New("foo").Parse(userTemplate)
			t.Execute(c.Writer, gothUser)
		} else {
			gothic.BeginAuthHandler(c.Writer, c.Request)
		}
	})

	r.GET("/", func(c *gin.Context) {
		t, _ := template.New("foo").Parse(indexTemplate)
		t.Execute(c.Writer, providerIndex)
	})

	r.Run(":3000")
}

type ProviderIndex struct {
	Providers    []string
	ProvidersMap map[string]string
}

var providerIndex *ProviderIndex

var indexTemplate = `{{range $key,$value:=.Providers}}
    <p><a href="/auth/{{$value}}">Log in with {{index $.ProvidersMap $value}}</a></p>
{{end}}`

var userTemplate = `
<p><a href="/logout/{{.Provider}}">logout</a></p>
<p>Name: {{.Name}} [{{.LastName}}, {{.FirstName}}]</p>
<p>Email: {{.Email}}</p>
<p>NickName: {{.NickName}}</p>
<p>Location: {{.Location}}</p>
<p>AvatarURL: {{.AvatarURL}} <img src="{{.AvatarURL}}"></p>
<p>Description: {{.Description}}</p>
<p>UserID: {{.UserID}}</p>
<p>ExpiresAt: {{.ExpiresAt}}</p>
<p>RefreshToken: {{.RefreshToken}}</p>
`
