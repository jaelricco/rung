package ai

import (
	"errors"
	"net/http"
	"strings"

	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/httpx"
)

// StatusNoProvider is what every coaching endpoint answers with when the
// caller has not connected an account yet. It is deliberately not 503: nothing
// is broken and nothing is switched off — the request is missing something
// only this athlete can supply, and the frontend turns it into a link to the
// settings page rather than an apology.
const StatusNoProvider = http.StatusPreconditionRequired

type connectionResponse struct {
	Connected  bool        `json:"connected"`
	Connection *Connection `json:"connection,omitempty"`
	Providers  []Provider  `json:"providers"`
	// KeystoreReady is false when the server itself was started without a
	// sealing key. Nobody can connect anything until that is fixed.
	KeystoreReady bool `json:"keystore_ready"`
}

// Connection reports what this athlete has connected, and what they could.
func (h *Handler) Connection(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())

	out := connectionResponse{Providers: Providers, KeystoreReady: h.store.Ready()}
	conn, ok, err := h.store.Connection(r.Context(), me.ID)
	if err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your AI settings.")
		return
	}
	if ok {
		out.Connected = true
		out.Connection = &conn
	}
	httpx.JSON(w, http.StatusOK, out)
}

type connectRequest struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

// Connect stores a key after proving it works. Sending only a model, with no
// key, switches the model on a connection that already exists.
func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	var in connectRequest
	if !httpx.Decode(w, r, &in) {
		return
	}
	me := auth.MustUser(r.Context())

	if !h.store.Ready() {
		httpx.Fail(w, http.StatusServiceUnavailable, ErrNoKeystore.Error())
		return
	}

	var (
		conn Connection
		err  error
	)
	if strings.TrimSpace(in.APIKey) == "" {
		// Switching model reuses the stored key; switching provider cannot.
		if stored, ok, _ := h.store.Connection(r.Context(), me.ID); ok &&
			in.Provider != "" && !strings.EqualFold(in.Provider, stored.Provider) {
			httpx.Fail(w, http.StatusBadRequest, "Paste a key for that provider to switch to it.")
			return
		}
		conn, err = h.store.UseModel(r.Context(), me.ID, in.Model)
		if errors.Is(err, ErrNoCredentials) {
			httpx.Fail(w, http.StatusBadRequest, "Paste your API key to connect an account.")
			return
		}
	} else {
		conn, err = h.store.Connect(r.Context(), me.ID, in.Provider, in.APIKey, in.Model)
	}
	if err != nil {
		// Everything reachable here is either the athlete's typo or their
		// provider's own refusal, and both are worth reading verbatim.
		httpx.Fail(w, http.StatusBadRequest, capitalise(err.Error()))
		return
	}

	httpx.JSON(w, http.StatusOK, connectionResponse{
		Connected:     true,
		Connection:    &conn,
		Providers:     Providers,
		KeystoreReady: true,
	})
}

// Disconnect forgets the key. The athlete should revoke it at their provider
// too, and the settings page says so.
func (h *Handler) Disconnect(w http.ResponseWriter, r *http.Request) {
	me := auth.MustUser(r.Context())
	if err := h.store.Disconnect(r.Context(), me.ID); err != nil {
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't remove that key. Try again.")
		return
	}
	httpx.JSON(w, http.StatusOK, connectionResponse{
		Providers:     Providers,
		KeystoreReady: h.store.Ready(),
	})
}

// FailNotConnected turns a store error into something worth reading on the
// page that hit it. Only the first case is the ordinary one: an athlete who
// has not connected an account yet.
func FailNotConnected(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoCredentials):
		httpx.Fail(w, StatusNoProvider, capitalise(ErrNoCredentials.Error())+".")
	case errors.Is(err, ErrNoKeystore):
		httpx.Fail(w, http.StatusServiceUnavailable, capitalise(ErrNoKeystore.Error())+".")
	default:
		httpx.Fail(w, http.StatusInternalServerError, "Couldn't read your stored API key: "+err.Error())
	}
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
