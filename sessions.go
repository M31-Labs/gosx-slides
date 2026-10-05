package slides

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"strings"

	"m31labs.dev/gosx/server"
	"m31labs.dev/gosx/session"
)

// SessionOptions enables an authenticated presentation room. Tokens are secrets,
// not identities: anyone holding the editor token has the same editor role.
type SessionOptions struct {
	AudienceToken string
	EditorToken   string
	// Secret optionally keeps sessions valid across restarts. An empty secret
	// creates a new key for this process. Explicit secrets require 32 bytes.
	Secret string
	// AllowInsecure permits cookies on plain HTTP, for trusted local networks.
	// The default requires HTTPS (a TLS reverse proxy must preserve HTTPS).
	AllowInsecure bool
}

type slidesSessionContextKey struct{}
type slidesSessionAccess struct{ role, csrf, id string }

func requestSession(r *http.Request) (slidesSessionAccess, bool) {
	if r == nil {
		return slidesSessionAccess{}, false
	}
	access, ok := r.Context().Value(slidesSessionContextKey{}).(slidesSessionAccess)
	return access, ok
}

// All source endpoints and WebSocket connections use the same writer policy.
func sourceRequestWriter(r *http.Request) bool {
	if access, enabled := requestSession(r); enabled {
		return access.role == "editor"
	}
	return r != nil && trustedSourceHost(r.Host)
}

func audienceSession(r *http.Request) bool {
	access, enabled := requestSession(r)
	return enabled && access.role == "audience"
}

func sessionRole(r *http.Request) string {
	access, _ := requestSession(r)
	return access.role
}

func sessionCSRF(r *http.Request) string {
	access, _ := requestSession(r)
	return access.csrf
}

func validateServeAccess(opts ServeOptions) error {
	if opts.Sessions != nil && opts.Static {
		return fmt.Errorf("sessions require a live server")
	}
	if opts.Addr != "" {
		host, _, err := net.SplitHostPort(opts.Addr)
		if err != nil {
			return fmt.Errorf("invalid listen address: %w", err)
		}
		local := strings.EqualFold(host, "localhost")
		if ip := net.ParseIP(host); ip != nil {
			local = ip.IsLoopback()
		}
		if !local && opts.Sessions == nil {
			return fmt.Errorf("non-loopback serving requires session tokens")
		}
	}
	return nil
}

func mountSessions(app *server.App, opts *SessionOptions, dir string) (*sessionGrants, error) {
	if opts == nil {
		return nil, nil
	}
	if len(opts.EditorToken) < 32 || len(opts.EditorToken) > 4096 {
		return nil, fmt.Errorf("editor token must contain 32–4096 bytes")
	}
	if opts.AudienceToken != "" && (len(opts.AudienceToken) < 32 || len(opts.AudienceToken) > 4096) {
		return nil, fmt.Errorf("audience token must contain 32–4096 bytes")
	}
	if opts.AudienceToken == opts.EditorToken {
		return nil, fmt.Errorf("audience and editor tokens must differ")
	}
	secret := opts.Secret
	if secret == "" {
		var key [32]byte
		if _, err := rand.Read(key[:]); err != nil {
			return nil, err
		}
		secret = hex.EncodeToString(key[:])
	} else if len(secret) < 32 {
		return nil, fmt.Errorf("session secret must contain at least 32 bytes")
	}
	manager, err := session.New(secret, session.Options{
		CookieName: "slides_session", MaxAge: presentationSessionAge,
		HTTPOnly: true, SameSite: http.SameSiteStrictMode, Encrypt: true,
		AllowInsecure: opts.AllowInsecure, LegacyCookieGrace: -1,
	})
	if err != nil {
		return nil, err
	}
	scopeInput, _ := json.Marshal([]string{secret, opts.EditorToken, opts.AudienceToken})
	scope := sha256.Sum256(scopeInput)
	grants, err := newSessionGrants(dir, hex.EncodeToString(scope[:]))
	if err != nil {
		return nil, err
	}
	editorHash := sha256.Sum256([]byte(opts.EditorToken))
	audienceHash := sha256.Sum256([]byte(opts.AudienceToken))
	hasAudience := opts.AudienceToken != ""
	app.Use(manager.Middleware)
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/_slides/session" || r.URL.Path == "/_slides/logout" {
				r.Body = http.MaxBytesReader(w, r.Body, 8192)
			}
			next.ServeHTTP(w, r)
		})
	})
	app.Use(manager.Protect)
	app.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			store := manager.Get(r)
			access := slidesSessionAccess{role: store.String("role"), csrf: manager.Token(r), id: store.String("room_id")}
			if !grants.valid(access.id, access.role) {
				access.role = ""
			}
			r = r.WithContext(context.WithValue(r.Context(), slidesSessionContextKey{}, access))
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Referrer-Policy", "same-origin")
			if r.URL.Path == "/_slides/session" || r.URL.Path == "/_slides/logout" {
				next.ServeHTTP(w, r)
				return
			}
			if access.role != "editor" && access.role != "audience" {
				if r.Method == http.MethodGet && (r.URL.Path == "/" || r.URL.Path == "/remote") {
					http.Redirect(w, r, "/_slides/session", http.StatusSeeOther)
				} else {
					http.Error(w, "presentation session required", http.StatusUnauthorized)
				}
				return
			}
			_, presenter := r.URL.Query()["present"]
			if access.role == "audience" && (strings.HasPrefix(r.URL.Path, "/_slides/") || r.URL.Path == "/remote" || r.URL.Path == "/presenter/state" || presenter) {
				http.Error(w, "editor session required", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	app.Mount("/_slides/session", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, sessionPage(manager.Token(r), ""))
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, 8192)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid session form", http.StatusBadRequest)
				return
			}
			digest := sha256.Sum256([]byte(r.PostForm.Get("token")))
			role := ""
			if subtle.ConstantTimeCompare(digest[:], editorHash[:]) == 1 {
				role = "editor"
			} else if hasAudience && subtle.ConstantTimeCompare(digest[:], audienceHash[:]) == 1 {
				role = "audience"
			}
			if role == "" {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				io.WriteString(w, sessionPage(manager.Token(r), "Token was not accepted."))
				return
			}
			store := manager.Get(r)
			id, err := grants.replace(store.String("room_id"), role)
			if err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
			store.Set("role", role)
			store.Set("room_id", id)
			manager.Token(r)
			http.Redirect(w, r, "/", http.StatusSeeOther)
		default:
			w.Header().Set("Allow", "GET, POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	app.Mount("/_slides/logout", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := grants.revoke(manager.Get(r).String("room_id")); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		manager.Get(r).Destroy()
		http.Redirect(w, r, "/_slides/session", http.StatusSeeOther)
	}))
	return grants, nil
}

func sessionPage(csrf, message string) string {
	return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Join presentation</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#111722;color:#edf2f7;font:1rem/1.6 system-ui}main{width:min(25rem,calc(100vw - 3rem))}h1{font-size:2rem}label,input,button{display:block}input,button{box-sizing:border-box;width:100%;font:inherit;padding:.75rem;margin:.5rem 0 1rem;border-radius:.5rem;border:1px solid #63718a}input{background:#1d2736;color:inherit}button{background:#ffd27d;color:#172031;cursor:pointer}p{color:#c4cedd}a{color:#ffd27d}:focus-visible{outline:3px solid #ffd27d;outline-offset:3px}</style></head><body><main><h1>Join presentation</h1><p>Use the audience or editor token provided by the host.</p><p role="status">` + html.EscapeString(message) + `</p><form method="post" action="/_slides/session"><input type="hidden" name="csrf_token" value="` + html.EscapeString(csrf) + `"><label for="token">Room token</label><input id="token" name="token" type="password" autocomplete="off" required maxlength="4096"><button>Join</button></form></main></body></html>`
}

const sessionHeadersScript = `window.SlidesSessionHeaders=function(headers){var result=Object.assign({},headers),meta=document.querySelector('meta[name="slides-csrf"]');if(meta&&meta.content)result['X-CSRF-Token']=meta.content;return result;};
document.addEventListener('DOMContentLoaded',function(){
  var controls=document.querySelector('.deck-controls'),meta=document.querySelector('meta[name="slides-csrf"]');if(!controls||!meta)return;
  var form=document.createElement('form');form.method='post';form.action='/_slides/logout';form.style.display='contents';
  var token=document.createElement('input');token.type='hidden';token.name='csrf_token';token.value=meta.content;form.appendChild(token);
  var button=document.createElement('button');button.type='submit';button.className='deck-icon-control';button.textContent='⇥';button.title='Leave presentation room';button.setAttribute('aria-label',button.title);form.appendChild(button);controls.appendChild(form);
});`
