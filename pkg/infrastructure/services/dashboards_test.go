// Copyright 2020 Red Hat, Inc. and/or its affiliates
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type serverHandler struct {
	Filename     string
	JSONResponse string
}

func Test_fetchDashboardNames(t *testing.T) {
	dashboardNames := `["dashboard1", "dashboard2"]`

	handlers := []serverHandler{
		serverHandler{
			Filename:     "list.json",
			JSONResponse: dashboardNames,
		},
	}

	server := mockKogitoSvcReplies(t, handlers)
	defer server.Close()

	dashboards, err := FetchGrafanaDashboardNamesForURL(server.URL)
	assert.NoError(t, err)
	assert.NotEmpty(t, dashboards)
}

func Test_fetchDashboards(t *testing.T) {
	dashboardNames := `["dashboard1", "dashboard2"]`
	dashboard1 := `mydashboard1`
	dashboard2 := `mydashboard2`

	handlers := []serverHandler{
		serverHandler{
			Filename:     "list.json",
			JSONResponse: dashboardNames,
		},
		serverHandler{
			Filename:     "dashboard1.json",
			JSONResponse: dashboard1,
		},
		serverHandler{
			Filename:     "dashboard2.json",
			JSONResponse: dashboard2,
		},
	}

	server := mockKogitoSvcReplies(t, handlers)
	defer server.Close()

	fetchedDashboardNames, err := FetchGrafanaDashboardNamesForURL(server.URL)
	assert.NoError(t, err)
	dashboards, err := FetchDashboards(server.URL, fetchedDashboardNames)
	assert.NoError(t, err)
	assert.Equal(t, len(fetchedDashboardNames), len(dashboards))
	assert.Equal(t, dashboard1, dashboards[0].RawJSONDashboard)
	assert.Equal(t, dashboard2, dashboards[1].RawJSONDashboard)
	assert.Equal(t, strings.ReplaceAll(dashboard2, ".json", ""), dashboards[0].Name)
}

func mockKogitoSvcReplies(t *testing.T, handlers []serverHandler) *httptest.Server {
	h := http.NewServeMux()
	for _, handler := range handlers {
		h.HandleFunc(dashboardsPath+handler.Filename, func(writer http.ResponseWriter, request *http.Request) {
			_, err := writer.Write([]byte(handler.JSONResponse))
			assert.NoError(t, err)
		})
	}

	return httptest.NewServer(h)
}
