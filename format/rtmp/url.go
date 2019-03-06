package rtmp

import (
	"fmt"
	"net/url"
	"strings"
)

func splitPath(u *url.URL) (app, stream string) {
	nu := *u
	nu.ForceQuery = false

	pathsegs := strings.SplitN(nu.RequestURI(), "/", -1)
	if len(pathsegs) == 2 {
		app = pathsegs[1]
	}
	if len(pathsegs) == 3 {
		app = pathsegs[1]
		stream = pathsegs[2]
	}
	if len(pathsegs) > 3 {
		app = strings.Join(pathsegs[1:3], "/")
		stream = strings.Join(pathsegs[3:], "/")
	}
	return
}

func getTcURL(u *url.URL) string {
	app, _ := splitPath(u)
	nu := *u
	nu.RawQuery = ""
	nu.Path = "/"
	return nu.String() + app
}

func createURL(tcurl, app, play string) (u *url.URL, err error) {
	if u, err = url.ParseRequestURI("/" + app + "/" + play); err != nil {
		return
	}

	var tu *url.URL
	if tu, err = url.Parse(tcurl); err != nil {
		return
	}

	if tu.Host == "" {
		err = fmt.Errorf("TcUrlHostEmpty")
		return
	}
	u.Host = tu.Host

	if tu.Scheme == "" {
		err = fmt.Errorf("TcUrlSchemeEmpty")
		return
	}
	u.Scheme = tu.Scheme

	return
}
