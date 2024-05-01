package httpmw

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/httpapi"
	"github.com/coder/coder/v2/codersdk"
)

type (
	intelCohortParamContextKey struct{}
)

// IntelCohortParam returns the cohort from the ExtractIntelCohortParam handler.
func IntelCohortParam(r *http.Request) database.IntelCohort {
	cohort, ok := r.Context().Value(intelCohortParamContextKey{}).(database.IntelCohort)
	if !ok {
		panic("developer error: intel cohort param middleware not provided")
	}
	return cohort
}

// ExtractIntelCohortParam grabs a cohort from the "cohort" URL parameter.
func ExtractIntelCohortParam(db database.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			arg := chi.URLParam(r, "cohort")
			if arg == "" {
				httpapi.Write(ctx, rw, http.StatusBadRequest, codersdk.Response{
					Message: "\"cohort\" must be provided.",
				})
				return
			}

			organization := OrganizationParam(r)

			cohorts, err := db.GetIntelCohortsByOrganizationID(ctx, database.GetIntelCohortsByOrganizationIDParams{
				OrganizationID: organization.ID,
				Name:           arg,
			})
			if err != nil || len(cohorts) == 0 {
				httpapi.Write(ctx, rw, http.StatusNotFound, codersdk.Response{
					Message: "cohort not found",
				})
				return
			}

			ctx = context.WithValue(ctx, intelCohortParamContextKey{}, cohorts[0])
			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	}
}
