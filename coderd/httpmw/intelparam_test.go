package httpmw_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/database/dbgen"
	"github.com/coder/coder/v2/coderd/database/dbmem"
	"github.com/coder/coder/v2/coderd/httpmw"
)

func TestIntelCohortParam(t *testing.T) {
	t.Parallel()
	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		var (
			db  = dbmem.New()
			rw  = httptest.NewRecorder()
			r   = httptest.NewRequest("GET", "/", nil)
			rtr = chi.NewRouter()
		)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
		organization := dbgen.Organization(t, db, database.Organization{})
		chi.RouteContext(r.Context()).URLParams.Add("organization", organization.ID.String())
		chi.RouteContext(r.Context()).URLParams.Add("cohort", "not-found")
		rtr.Use(
			httpmw.ExtractOrganizationParam(db),
			httpmw.ExtractIntelCohortParam(db),
		)
		rtr.Get("/", nil)
		rtr.ServeHTTP(rw, r)
		res := rw.Result()
		defer res.Body.Close()
		require.Equal(t, http.StatusNotFound, res.StatusCode)
	})
	t.Run("Found", func(t *testing.T) {
		t.Parallel()
		var (
			db  = dbmem.New()
			rw  = httptest.NewRecorder()
			r   = httptest.NewRequest("GET", "/", nil)
			rtr = chi.NewRouter()
		)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
		organization := dbgen.Organization(t, db, database.Organization{})
		cohort := dbgen.IntelCohort(t, db, database.IntelCohort{OrganizationID: organization.ID})
		chi.RouteContext(r.Context()).URLParams.Add("organization", organization.ID.String())
		chi.RouteContext(r.Context()).URLParams.Add("cohort", cohort.Name)
		rtr.Use(
			httpmw.ExtractOrganizationParam(db),
			httpmw.ExtractIntelCohortParam(db),
		)
		rtr.Get("/", func(w http.ResponseWriter, r *http.Request) {
			rw.WriteHeader(http.StatusOK)
			got := httpmw.IntelCohortParam(r)
			require.Equal(t, cohort.ID, got.ID)
		})
		rtr.ServeHTTP(rw, r)
		res := rw.Result()
		defer res.Body.Close()
		require.Equal(t, http.StatusOK, res.StatusCode)
	})
}
