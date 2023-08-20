package route

import (
	"context"
	"math"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/miekg/dns"
	"github.com/samber/lo"

	"github.com/Dreamacro/clash/common/util"
	"github.com/Dreamacro/clash/component/resolver"
)

func dnsRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/query", queryDNS)
	return r
}

func queryDNS(w http.ResponseWriter, r *http.Request) {
	if resolver.DefaultResolver == nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, newError("DNS section is disabled"))
		return
	}

	var (
		name     = r.URL.Query().Get("name")
		proxy    = r.URL.Query().Get("proxy")
		qTypeStr = util.EmptyOr(r.URL.Query().Get("type"), "A")
	)

	qType, exist := dns.StringToType[strings.ToUpper(qTypeStr)]
	if !exist {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, newError("invalid query type"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolver.DefaultDNSTimeout)
	defer cancel()

	if proxy != "" {
		ctx = resolver.WithProxy(ctx, proxy)
	}

	msg := dns.Msg{}
	msg.SetQuestion(dns.Fqdn(name), qType)
	resp, source, err := resolver.DefaultResolver.ExchangeContext(ctx, &msg)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, newError(err.Error()))
		return
	}

	responseData := render.M{
		"Server":   source,
		"Status":   resp.Rcode,
		"Question": resp.Question,
		"TC":       resp.Truncated,
		"RD":       resp.RecursionDesired,
		"RA":       resp.RecursionAvailable,
		"AD":       resp.AuthenticatedData,
		"CD":       resp.CheckingDisabled,
	}

	rr2Json := func(rr dns.RR, _ int) render.M {
		header := rr.Header()
		return render.M{
			"name": header.Name,
			"type": header.Rrtype,
			"TTL":  header.Ttl,
			"data": lo.Substring(rr.String(), len(header.String()), math.MaxUint),
		}
	}

	if len(resp.Answer) > 0 {
		responseData["Answer"] = lo.Map(resp.Answer, rr2Json)
	}
	if len(resp.Ns) > 0 {
		responseData["Authority"] = lo.Map(resp.Ns, rr2Json)
	}
	if len(resp.Extra) > 0 {
		responseData["Additional"] = lo.Map(resp.Extra, rr2Json)
	}

	render.JSON(w, r, responseData)
}
