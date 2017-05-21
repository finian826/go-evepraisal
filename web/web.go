package web

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/evepraisal/go-evepraisal"
	"github.com/husobee/vestigo"
	"github.com/mash/go-accesslog"
)

type accessLogger struct {
}

func (l accessLogger) Log(record accesslog.LogRecord) {
	log.Printf("%s %s %d (%s) - %d", record.Method, record.Uri, record.Status, record.Ip, record.Size)
}

var templateFuncs = template.FuncMap{
	"commaf":          humanizeCommaf,
	"comma":           humanize.Comma,
	"prettybignumber": HumanLargeNumber,
	"relativetime":    humanize.Time,
	"timefmt":         func(t time.Time) string { return t.Format("2006-01-02 15:04:05") },

	// Only for debugging
	"spew": spew.Sdump,
}

type MainPageStruct struct {
	Appraisal           *evepraisal.Appraisal
	TotalAppraisalCount int64
}

func (ctx *Context) HandleIndex(w http.ResponseWriter, r *http.Request) {
	txn := ctx.app.TransactionLogger.StartWebTransaction("view_index", w, r)
	defer txn.End()

	total, err := ctx.app.AppraisalDB.TotalAppraisals()
	if err != nil {
		ctx.renderErrorPage(w, http.StatusInternalServerError, "Something bad happened", err.Error())
		return
	}
	ctx.render(w, "main.html", MainPageStruct{TotalAppraisalCount: total})
}

func (ctx *Context) HandleAppraisal(w http.ResponseWriter, r *http.Request) {
	txn := ctx.app.TransactionLogger.StartWebTransaction("create_appraisal", w, r)
	defer txn.End()
	if len(r.FormValue("body")) > 200000 {
		ctx.renderErrorPage(w, http.StatusBadRequest, "Invalid input", "Input value is too big.")
		return
	}

	appraisal, err := ctx.app.StringToAppraisal(r.FormValue("market"), r.FormValue("body"))
	if err != nil {
		ctx.renderErrorPage(w, http.StatusBadRequest, "Invalid input", err.Error())
		return
	}

	err = ctx.app.AppraisalDB.PutNewAppraisal(appraisal)
	if err != nil {
		log.Printf("ERROR: saving appraisal: %s", err)
		ctx.renderErrorPage(w, http.StatusInternalServerError, "Something bad happened", err.Error())
		return
	}
	log.Printf("[New appraisal] id=%s, market=%s, items=%d, unparsed=%d", appraisal.ID, appraisal.MarketName, len(appraisal.Items), len(appraisal.Unparsed))

	err = ctx.render(w, "main.html", MainPageStruct{Appraisal: appraisal})
}

func (ctx *Context) HandleViewAppraisal(w http.ResponseWriter, r *http.Request) {
	txn := ctx.app.TransactionLogger.StartWebTransaction("view_appraisal", w, r)
	defer txn.End()

	appraisalID := vestigo.Param(r, "appraisalID")
	if strings.HasSuffix(appraisalID, ".json") {
		ctx.HandleViewAppraisalJSON(w, r)
		return
	}

	if strings.HasSuffix(appraisalID, ".raw") {
		ctx.HandleViewAppraisalRAW(w, r)
		return
	}

	appraisal, err := ctx.app.AppraisalDB.GetAppraisal(appraisalID)
	if err == evepraisal.AppraisalNotFound {
		ctx.renderErrorPage(w, http.StatusNotFound, "Not Found", "I couldn't find what you're looking for")
		return
	} else if err != nil {
		ctx.renderErrorPage(w, http.StatusInternalServerError, "Something bad happened", err.Error())
		return
	}

	sort.Slice(appraisal.Items, func(i, j int) bool {
		return appraisal.Items[i].SingleRepresentativePrice() > appraisal.Items[j].SingleRepresentativePrice()
	})

	ctx.render(w, "main.html", MainPageStruct{Appraisal: appraisal})
}

func (ctx *Context) HandleViewAppraisalJSON(w http.ResponseWriter, r *http.Request) {
	txn := ctx.app.TransactionLogger.StartWebTransaction("view_appraisal_json", w, r)
	defer txn.End()

	appraisalID := vestigo.Param(r, "appraisalID")
	appraisalID = strings.TrimSuffix(appraisalID, ".json")

	appraisal, err := ctx.app.AppraisalDB.GetAppraisal(appraisalID)
	if err == evepraisal.AppraisalNotFound {
		ctx.renderErrorPage(w, http.StatusNotFound, "Not Found", "I couldn't find what you're looking for")
		return
	} else if err != nil {
		ctx.renderErrorPage(w, http.StatusInternalServerError, "Something bad happened", err.Error())
		return
	}

	r.Header["Content-Type"] = []string{"application/json"}
	json.NewEncoder(w).Encode(appraisal)
}

func (ctx *Context) HandleViewAppraisalRAW(w http.ResponseWriter, r *http.Request) {
	txn := ctx.app.TransactionLogger.StartWebTransaction("view_appraisal_raw", w, r)
	defer txn.End()

	appraisalID := vestigo.Param(r, "appraisalID")
	appraisalID = strings.TrimSuffix(appraisalID, ".raw")

	appraisal, err := ctx.app.AppraisalDB.GetAppraisal(appraisalID)
	if err == evepraisal.AppraisalNotFound {
		ctx.renderErrorPage(w, http.StatusNotFound, "Not Found", "I couldn't find what you're looking for")
		return
	} else if err != nil {
		ctx.renderErrorPage(w, http.StatusInternalServerError, "Something bad happened", err.Error())
		return
	}

	r.Header["Content-Type"] = []string{"application/text"}
	io.WriteString(w, appraisal.Raw)
}

func (ctx *Context) HandleLatestAppraisals(w http.ResponseWriter, r *http.Request) {
	txn := ctx.app.TransactionLogger.StartWebTransaction("view_latest_appraisals", w, r)
	defer txn.End()

	var limit int64
	var err error
	limit, err = strconv.ParseInt(r.FormValue("limit"), 10, 64)
	if err != nil {
		limit = 100
	}

	appraisals, err := ctx.app.AppraisalDB.LatestAppraisals(int(limit), r.FormValue("kind"))
	if err != nil {
		ctx.renderErrorPage(w, http.StatusInternalServerError, "Something bad happened", err.Error())
		return
	}

	ctx.render(w, "latest.html", struct{ Appraisals []evepraisal.Appraisal }{appraisals})
}

func (ctx *Context) HandleLegal(w http.ResponseWriter, r *http.Request) {
	txn := ctx.app.TransactionLogger.StartWebTransaction("view_legal", w, r)
	defer txn.End()
	ctx.render(w, "legal.html", "wat")
}