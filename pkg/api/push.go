package api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/flanksource/commons/logger"
)

type QueueData struct {
	Check  pkg.Check       `json:",inline"`
	Status pkg.CheckStatus `json:",inline"`
}

func PushHandler(c echo.Context) error {
	if c.Request().Body == nil {
		logger.Debugf("missing request body")
		return errorResponse(c, errors.New("missing request body"), http.StatusBadRequest)
	}
	defer c.Request().Body.Close()

	ctx := c.Request().Context().(dutyContext.Context)

	data := QueueData{
		Check:  pkg.Check{},
		Status: pkg.CheckStatus{},
	}
	reqBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return errorResponse(c, err, http.StatusInternalServerError)
	}
	if err := json.Unmarshal(reqBody, &data); err != nil {
		return errorResponse(c, err, http.StatusBadRequest)
	}
	if data.Check.ID != uuid.Nil && data.Check.CanaryID == uuid.Nil {
		check, err := db.GetCheck(ctx, data.Check.ID.String())
		if check == nil && err == nil {
			return errorResponse(c, errors.New("check not found"), http.StatusNotFound)
		} else if err != nil {
			return errorResponse(c, err, http.StatusInternalServerError)
		}
		data.Check.CanaryID = check.CanaryID
	} else if data.Check.ID == uuid.Nil {
		canary, err := db.FindCanary(ctx, data.Check.Namespace, data.Check.Name)
		if err != nil {
			return errorResponse(c, err, http.StatusInternalServerError)
		}
		if canary != nil {
			data.Check.CanaryID = canary.ID
		} else {
			canary = &pkg.Canary{
				Name:      data.Check.Name,
				Namespace: data.Check.Namespace,
			}
			if err := db.CreateCanary(ctx, canary); err != nil {
				return errorResponse(c, err, http.StatusInternalServerError)
			}
			data.Check.CanaryID = canary.ID
			if err := db.CreateCheck(ctx, *canary, &data.Check); err != nil {
				return errorResponse(c, err, http.StatusInternalServerError)
			}
		}
	}
	cache.PostgresCache.Add(data.Check, data.Status)
	c.Response().WriteHeader(http.StatusCreated)
	return nil
}

func PostDataToServer(server string, body io.Reader) (err error) {
	url := fmt.Sprintf("%s/api/push", server)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Post(url, "application/json", body)
	if err != nil {
		return err
	}
	defer func() { err = resp.Body.Close() }()
	responseBody, _ := io.ReadAll(resp.Body)
	logger.Tracef("[%s] %d %s", server, resp.StatusCode, responseBody)
	if resp.StatusCode != 201 {
		return errors.New(string(responseBody))
	}
	return err
}
