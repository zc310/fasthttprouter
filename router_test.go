// Copyright 2013 Julien Schmidt. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package fasthttprouter

import (
	"bytes"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/valyala/fasthttp"

	"io/ioutil"
	"os"
)

var NewRequest = func(method, url string, body io.Reader) (*fasthttp.RequestCtx, error) {
	var ctx fasthttp.RequestCtx
	ctx.Request.Header.SetHost("www.aaa.com")
	ctx.Request.Header.SetMethod(method)
	ctx.Request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.78 Safari/537.36")
	ctx.Request.Header.Set("Referer", "http://www.baidu.com/")
	ctx.Request.SetRequestURI(url)
	return &ctx, nil
}

func TestRouter(t *testing.T) {
	router := New()

	routed := false
	router.Handle("GET", "/user/:name", func(ctx *fasthttp.RequestCtx) {
		routed = true
		want := "gopher"
		if !reflect.DeepEqual(ctx.UserValue("name"), want) {
			t.Fatalf("wrong wildcard values: want %v, got %v", want, ctx.UserValue("name"))
		}
	})

	r, _ := NewRequest("GET", "/user/gopher", nil)
	router.Handler(r)

	if !routed {
		t.Fatal("routing failed")
	}
}

func TestRouterAPI(t *testing.T) {
	var get, head, options, post, put, patch, delete bool

	router := New()
	router.GET("/GET", func(ctx *fasthttp.RequestCtx) {
		get = true
	})
	router.HEAD("/GET", func(ctx *fasthttp.RequestCtx) {
		head = true
	})
	router.OPTIONS("/GET", func(ctx *fasthttp.RequestCtx) {
		options = true
	})
	router.POST("/POST", func(ctx *fasthttp.RequestCtx) {
		post = true
	})
	router.PUT("/PUT", func(ctx *fasthttp.RequestCtx) {
		put = true
	})
	router.PATCH("/PATCH", func(ctx *fasthttp.RequestCtx) {
		patch = true
	})
	router.DELETE("/DELETE", func(ctx *fasthttp.RequestCtx) {
		delete = true
	})

	r, _ := NewRequest("GET", "/GET", nil)
	router.Handler(r)
	if !get {
		t.Error("routing GET failed")
	}

	r, _ = NewRequest("HEAD", "/GET", nil)
	router.Handler(r)
	if !head {
		t.Error("routing HEAD failed")
	}

	r, _ = NewRequest("OPTIONS", "/GET", nil)
	router.Handler(r)
	if !options {
		t.Error("routing OPTIONS failed")
	}

	r, _ = NewRequest("POST", "/POST", nil)
	router.Handler(r)
	if !post {
		t.Error("routing POST failed")
	}

	r, _ = NewRequest("PUT", "/PUT", nil)
	router.Handler(r)
	if !put {
		t.Error("routing PUT failed")
	}

	r, _ = NewRequest("PATCH", "/PATCH", nil)
	router.Handler(r)
	if !patch {
		t.Error("routing PATCH failed")
	}

	r, _ = NewRequest("DELETE", "/DELETE", nil)
	router.Handler(r)
	if !delete {
		t.Error("routing DELETE failed")
	}
}

func TestRouterRoot(t *testing.T) {
	router := New()
	recv := catchPanic(func() {
		router.GET("noSlashRoot", nil)
	})
	if recv == nil {
		t.Fatal("registering path not beginning with '/' did not panic")
	}
}

func TestRouterChaining(t *testing.T) {
	router1 := New()
	router2 := New()
	router1.NotFound = router2.Handler

	fooHit := false
	router1.POST("/foo", func(ctx *fasthttp.RequestCtx) {
		fooHit = true
		ctx.SetStatusCode(http.StatusOK)
	})

	barHit := false
	router2.POST("/bar", func(ctx *fasthttp.RequestCtx) {
		barHit = true
		ctx.SetStatusCode(http.StatusOK)
	})

	r, _ := NewRequest("POST", "/foo", nil)

	router1.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK && fooHit) {
		t.Errorf("Regular routing failed with router chaining.")
		t.FailNow()
	}

	r, _ = NewRequest("POST", "/bar", nil)
	router1.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK && barHit) {
		t.Errorf("Chained routing failed with router chaining.")
		t.FailNow()
	}

	r, _ = NewRequest("POST", "/qax", nil)
	router1.Handler(r)
	if !(r.Response.StatusCode() == http.StatusNotFound) {
		t.Errorf("NotFound behavior failed with router chaining.")
		t.FailNow()
	}
}

func TestRouterOPTIONS(t *testing.T) {
	handlerFunc := func(ctx *fasthttp.RequestCtx) {}

	router := New()
	router.POST("/path", handlerFunc)

	// test not allowed
	// * (server)
	r, _ := NewRequest("OPTIONS", "*", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	} else if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	r, _ = NewRequest("OPTIONS", "/path", nil)

	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	} else if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	r, _ = NewRequest("OPTIONS", "/doesnotexist", nil)

	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusNotFound) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	}

	// add another method
	router.GET("/path", handlerFunc)

	// test again
	// * (server)
	r, _ = NewRequest("OPTIONS", "*", nil)

	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	} else if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	r, _ = NewRequest("OPTIONS", "/path", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	} else if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// custom handler
	var custom bool
	router.OPTIONS("/path", func(ctx *fasthttp.RequestCtx) {
		custom = true
	})

	// test again
	// * (server)
	r, _ = NewRequest("OPTIONS", "*", nil)

	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	} else if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}
	if custom {
		t.Error("custom handler called on *")
	}

	// path
	r, _ = NewRequest("OPTIONS", "/path", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	}
	if !custom {
		t.Error("custom handler not called")
	}
}

func TestRouterNotAllowed(t *testing.T) {
	handlerFunc := func(ctx *fasthttp.RequestCtx) {}

	router := New()
	router.POST("/path", handlerFunc)
	// test not allowed
	r, _ := NewRequest("GET", "/path", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	} else if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// add another method
	router.DELETE("/path", handlerFunc)
	router.OPTIONS("/path", handlerFunc) // must be ignored

	// test again
	r, _ = NewRequest("GET", "/path", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == http.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	} else if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, DELETE, OPTIONS" && allow != "DELETE, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// test custom handler
	r.Response.Reset()
	responseText := "custom method"
	router.MethodNotAllowed = fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusTeapot)
		ctx.Write([]byte(responseText))
	})
	router.Handler(r)
	if got := string(r.Response.Body()); !(got == responseText) {
		t.Errorf("unexpected response got %q want %q", got, responseText)
	}
	if r.Response.StatusCode() != http.StatusTeapot {
		t.Errorf("unexpected response code %d want %d", r.Response.StatusCode(), http.StatusTeapot)
	}
	if allow := string(r.Response.Header.Peek("Allow")); allow != "POST, DELETE, OPTIONS" && allow != "DELETE, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}
}

func TestRouterNotFound(t *testing.T) {
	handlerFunc := func(ctx *fasthttp.RequestCtx) {}

	router := New()
	router.GET("/path", handlerFunc)
	router.GET("/dir/", handlerFunc)
	router.GET("/", handlerFunc)

	testRoutes := []struct {
		route  string
		code   int
		header string
	}{
		{"/path/", 301, "http://www.aaa.com/path"}, // TSR -/
		{"/dir", 301, "http://www.aaa.com/dir/"},   // TSR +/
		{"", 200, ""},                              // TSR +/
		{"/PATH", 301, "http://www.aaa.com/path"},  // Fixed Case
		{"/DIR/", 301, "http://www.aaa.com/dir/"},  // Fixed Case
		{"/PATH/", 301, "http://www.aaa.com/path"}, // Fixed Case -/
		{"/DIR", 301, "http://www.aaa.com/dir/"},   // Fixed Case +/
		{"/../path", 200, ""},                      // CleanPath
		{"/nope", 404, ""},                         // NotFound
	}
	for _, tr := range testRoutes {
		r, _ := NewRequest("GET", tr.route, nil)
		router.Handler(r)

		if !(r.Response.StatusCode() == tr.code && (r.Response.StatusCode() == 404 || string(r.Response.Header.Peek("Location")) == tr.header)) {
			t.Errorf("NotFound handling route %s failed: Code=%d, Location=%v", tr.route, r.Response.StatusCode(), string(r.Response.Header.Peek("Location")))
		}
	}

	// Test custom not found handler
	var notFound bool
	router.NotFound = fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(404)
		notFound = true
	})
	r, _ := NewRequest("GET", "/nope", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == 404 && notFound == true) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	}

	// Test other method than GET (want 307 instead of 301)
	router.PATCH("/path", handlerFunc)
	r, _ = NewRequest("PATCH", "/path/", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == 307 && string(r.Response.Header.Peek("Location")) == "http://www.aaa.com/path") {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", r.Response.StatusCode(), r.Response.Header.String())
	}

	// Test special case where no node for the prefix "/" exists
	router = New()
	router.GET("/a", handlerFunc)
	r, _ = NewRequest("GET", "/", nil)
	router.Handler(r)
	if !(r.Response.StatusCode() == 404) {
		t.Errorf("NotFound handling route / failed: Code=%d", r.Response.StatusCode())
	}
}

func TestRouterPanicHandler(t *testing.T) {
	router := New()
	panicHandled := false

	router.PanicHandler = func(ctx *fasthttp.RequestCtx, p interface{}) {
		panicHandled = true
	}

	router.Handle("PUT", "/user/:name", func(ctx *fasthttp.RequestCtx) {
		panic("oops!")
	})

	req, _ := NewRequest("PUT", "/user/gopher", nil)

	defer func() {
		if rcv := recover(); rcv != nil {
			t.Fatal("handling panic failed")
		}
	}()

	router.Handler(req)

	if !panicHandled {
		t.Fatal("simulating failed")
	}
}

func TestRouterLookup(t *testing.T) {
	routed := false
	wantHandle := func(ctx *fasthttp.RequestCtx) {
		routed = true
	}

	router := New()
	ctx := &fasthttp.RequestCtx{}

	// try empty router first
	handle, tsr := router.Lookup("GET", "/nope", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}

	// insert route and try again
	router.GET("/user/:name", wantHandle)

	handle, tsr = router.Lookup("GET", "/user/gopher", ctx)
	if handle == nil {
		t.Fatal("Got no handle!")
	} else {
		handle(nil)
		if !routed {
			t.Fatal("Routing failed!")
		}
	}

	if !reflect.DeepEqual(ctx.UserValue("name"), "gopher") {
		t.Fatalf("Wrong parameter values: want %v, got %v", ctx.UserValue("name"), "name")
	}

	handle, tsr = router.Lookup("GET", "/user/gopher/", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if !tsr {
		t.Error("Got no TSR recommendation!")
	}

	handle, tsr = router.Lookup("GET", "/nope", ctx)
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}
}

func TestRouterServeFiles(t *testing.T) {
	router := New()

	recv := catchPanic(func() {
		router.ServeFiles("/noFilepath", os.TempDir())
	})
	if recv == nil {
		t.Fatal("registering path not ending with '*filepath' did not panic")
	}

	body := []byte("ico")
	ioutil.WriteFile(os.TempDir()+"/favicon.ico", body, 0644)

	router.ServeFiles("/*filepath", os.TempDir())

	r, _ := NewRequest("GET", "/favicon.ico", nil)
	router.Handler(r)
	if !bytes.Equal(body, r.Response.Body()) {
		t.Error("serving file failed")
	}
}
