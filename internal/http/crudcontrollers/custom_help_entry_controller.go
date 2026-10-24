package crudcontrollers

import (
	"fmt"
	"github.com/EQEmu/spire/internal/auditlog"
	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"strconv"
	"strings"
)

type CustomHelpEntryController struct {
	db       *database.Resolver
	auditLog *auditlog.UserEvent
}

func NewCustomHelpEntryController(
	db *database.Resolver,
	auditLog *auditlog.UserEvent,
) *CustomHelpEntryController {
	return &CustomHelpEntryController{
		db:       db,
		auditLog: auditLog,
	}
}

func (e *CustomHelpEntryController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "custom_help_entry/:id", e.getCustomHelpEntry, nil),
		routes.RegisterRoute(http.MethodGet, "custom_help_entries", e.listCustomHelpEntries, nil),
		routes.RegisterRoute(http.MethodGet, "custom_help_entries/count", e.getCustomHelpEntriesCount, nil),
		routes.RegisterRoute(http.MethodPut, "custom_help_entry", e.createCustomHelpEntry, nil),
		routes.RegisterRoute(http.MethodDelete, "custom_help_entry/:id", e.deleteCustomHelpEntry, nil),
		routes.RegisterRoute(http.MethodPatch, "custom_help_entry/:id", e.updateCustomHelpEntry, nil),
		routes.RegisterRoute(http.MethodPost, "custom_help_entries/bulk", e.getCustomHelpEntriesBulk, nil),
	}
}

// listCustomHelpEntries godoc
// @Id listCustomHelpEntries
// @Summary Lists CustomHelpEntries
// @Accept json
// @Produce json
// @Tags CustomHelpEntry
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomHelpEntry
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_entries [get]
func (e *CustomHelpEntryController) listCustomHelpEntries(c echo.Context) error {
	var results []models.CustomHelpEntry
	err := e.db.QueryContext(models.CustomHelpEntry{}, c).Find(&results).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomHelpEntry godoc
// @Id getCustomHelpEntry
// @Summary Gets CustomHelpEntry
// @Accept json
// @Produce json
// @Tags CustomHelpEntry
// @Param id path int true "Id"
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomHelpEntry
// @Failure 404 {string} string "Entity not found"
// @Failure 500 {string} string "Cannot find param"
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_entry/{id} [get]
func (e *CustomHelpEntryController) getCustomHelpEntry(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [Id]"})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomHelpEntry
	query := e.db.QueryContext(models.CustomHelpEntry{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	// couldn't find entity
	if result.ID == 0 {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Cannot find entity"})
	}

	return c.JSON(http.StatusOK, result)
}

// updateCustomHelpEntry godoc
// @Id updateCustomHelpEntry
// @Summary Updates CustomHelpEntry
// @Accept json
// @Produce json
// @Tags CustomHelpEntry
// @Param id path int true "Id"
// @Param custom_help_entry body models.CustomHelpEntry true "CustomHelpEntry"
// @Success 200 {array} models.CustomHelpEntry
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error updating entity"
// @Router /custom_help_entry/{id} [patch]
func (e *CustomHelpEntryController) updateCustomHelpEntry(c echo.Context) error {
	request := new(models.CustomHelpEntry)
	if err := c.Bind(request); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Cannot find param [Id]"})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomHelpEntry
	query := e.db.QueryContext(models.CustomHelpEntry{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Cannot find entity [%s]", err.Error())})
	}

	// save top-level using only changes
	diff := database.ResultDifference(result, request)
	err = query.Session(&gorm.Session{FullSaveAssociations: false}).Updates(diff).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": fmt.Sprintf("Error updating entity [%v]", err.Error())})
	}

	// log update event
	if e.db.GetSpireDb() != nil && len(diff) > 0 {
		// build ids
		var ids []string
		for i, _ := range keys {
			param := fmt.Sprintf("%v", params[i])
			ids = append(ids, fmt.Sprintf("%v", strings.ReplaceAll(keys[i], "?", param)))
		}
		// build fields updated
		var fieldsUpdated []string
		for k, v := range diff {
			fieldsUpdated = append(fieldsUpdated, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Updated [CustomHelpEntry] [%v] fields [%v]", strings.Join(ids, ", "), strings.Join(fieldsUpdated, ", "))
		e.auditLog.LogUserEvent(c, "UPDATE", event)
	}

	return c.JSON(http.StatusOK, request)
}

// createCustomHelpEntry godoc
// @Id createCustomHelpEntry
// @Summary Creates CustomHelpEntry
// @Accept json
// @Produce json
// @Param custom_help_entry body models.CustomHelpEntry true "CustomHelpEntry"
// @Tags CustomHelpEntry
// @Success 200 {array} models.CustomHelpEntry
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error inserting entity"
// @Router /custom_help_entry [put]
func (e *CustomHelpEntryController) createCustomHelpEntry(c echo.Context) error {
	customHelpEntry := new(models.CustomHelpEntry)
	if err := c.Bind(customHelpEntry); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to entity [%v]", err.Error())},
		)
	}

	db := e.db.Get(models.CustomHelpEntry{}, c).Model(&models.CustomHelpEntry{})

	// save associations
	if c.QueryParam("save_associations") != "true" {
		db = db.Omit(clause.Associations)
	}

	err := db.Create(&customHelpEntry).Error
	if err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error inserting entity [%v]", err.Error())},
		)
	}

	// log create event
	if e.db.GetSpireDb() != nil {
		// diff between an empty model and the created
		diff := database.ResultDifference(models.CustomHelpEntry{}, customHelpEntry)
		// build fields created
		var fields []string
		for k, v := range diff {
			fields = append(fields, fmt.Sprintf("%v = %v", k, v))
		}
		// record event
		event := fmt.Sprintf("Created [CustomHelpEntry] [%v] data [%v]", customHelpEntry.ID, strings.Join(fields, ", "))
		e.auditLog.LogUserEvent(c, "CREATE", event)
	}

	return c.JSON(http.StatusOK, customHelpEntry)
}

// deleteCustomHelpEntry godoc
// @Id deleteCustomHelpEntry
// @Summary Deletes CustomHelpEntry
// @Accept json
// @Produce json
// @Tags CustomHelpEntry
// @Param id path int true "id"
// @Success 200 {string} string "Entity deleted successfully"
// @Failure 404 {string} string "Cannot find entity"
// @Failure 500 {string} string "Error binding to entity"
// @Failure 500 {string} string "Error deleting entity"
// @Router /custom_help_entry/{id} [delete]
func (e *CustomHelpEntryController) deleteCustomHelpEntry(c echo.Context) error {
	var params []interface{}
	var keys []string

	// primary key param
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	params = append(params, id)
	keys = append(keys, "id = ?")

	// query builder
	var result models.CustomHelpEntry
	query := e.db.QueryContext(models.CustomHelpEntry{}, c)
	for i, _ := range keys {
		query = query.Where(keys[i], params[i])
	}

	// grab first entry
	err = query.First(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	err = query.Limit(10000).Delete(&result).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Error deleting entity"})
	}

	// log delete event
	if e.db.GetSpireDb() != nil {
		// build ids
		var ids []string
		for i, _ := range keys {
			param := fmt.Sprintf("%v", params[i])
			ids = append(ids, fmt.Sprintf("%v", strings.ReplaceAll(keys[i], "?", param)))
		}
		// record event
		event := fmt.Sprintf("Deleted [CustomHelpEntry] [%v] keys [%v]", result.ID, strings.Join(ids, ", "))
		e.auditLog.LogUserEvent(c, "DELETE", event)
	}

	return c.JSON(http.StatusOK, echo.Map{"success": "Entity deleted successfully"})
}

// getCustomHelpEntriesBulk godoc
// @Id getCustomHelpEntriesBulk
// @Summary Gets CustomHelpEntries in bulk
// @Accept json
// @Produce json
// @Param Body body BulkFetchByIdsGetRequest true "body"
// @Tags CustomHelpEntry
// @Success 200 {array} models.CustomHelpEntry
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_entries/bulk [post]
func (e *CustomHelpEntryController) getCustomHelpEntriesBulk(c echo.Context) error {
	var results []models.CustomHelpEntry

	r := new(BulkFetchByIdsGetRequest)
	if err := c.Bind(r); err != nil {
		return c.JSON(
			http.StatusInternalServerError,
			echo.Map{"error": fmt.Sprintf("Error binding to bulk request: [%v]", err.Error())},
		)
	}

	if len(r.IDs) == 0 {
		return c.JSON(
			http.StatusOK,
			echo.Map{"error": fmt.Sprintf("Missing request field data 'ids'")},
		)
	}

	err := e.db.QueryContext(models.CustomHelpEntry{}, c).Find(&results, r.IDs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, results)
}

// getCustomHelpEntriesCount godoc
// @Id getCustomHelpEntriesCount
// @Summary Counts CustomHelpEntries
// @Accept json
// @Produce json
// @Tags CustomHelpEntry
// @Param includes query string false "Relationships [all] for all [number] for depth of relationships to load or [.] separated relationship names"
// @Param where query string false "Filter on specific fields. Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param whereOr query string false "Filter on specific fields (Chained ors). Multiple conditions [.] separated Example: col_like_value.col2__val2"
// @Param groupBy query string false "Group by field. Multiple conditions [.] separated Example: field1.field2"
// @Param limit query string false "Rows to limit in response (Default: 10,000)"
// @Param page query int 0 "Pagination page"
// @Param orderBy query string false "Order by [field]"
// @Param orderDirection query string false "Order by field direction"
// @Param select query string false "Column names [.] separated to fetch specific fields in response"
// @Success 200 {array} models.CustomHelpEntry
// @Failure 500 {string} string "Bad query request"
// @Router /custom_help_entries/count [get]
func (e *CustomHelpEntryController) getCustomHelpEntriesCount(c echo.Context) error {
	var count int64
	err := e.db.QueryContext(models.CustomHelpEntry{}, c).Count(&count).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"count": count})
}