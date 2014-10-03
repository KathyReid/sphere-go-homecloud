package routes

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-martini/martini"
	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/sphere-go-homecloud/homecloud"
)

type ThingRouter struct {
}

func NewThingRouter() *ThingRouter {
	return &ThingRouter{}
}

func (lr *ThingRouter) Register(r martini.Router) {
	r.Get("/", lr.GetAll)
	r.Get("/:id", lr.GetThing)
	r.Put("/:id", lr.PutThing)
	r.Put("/:id/location", lr.PutThingLocation)
	r.Delete("/:id", lr.DeleteThing)

}

func (lr *ThingRouter) GetAll(r *http.Request, w http.ResponseWriter, thingModel *homecloud.ThingModel) {
	// if type is specified as a query param
	qs := r.URL.Query()

	var err error
	var things *[]*model.Thing

	if qs.Get("type") != "" {
		things, err = thingModel.FetchByType(qs.Get("type"))
	} else {
		things, err = thingModel.FetchAll()
	}

	if err != nil {
		WriteServerErrorResponse("Unable to retrieve things", http.StatusInternalServerError, w)
		return
	}

	WriteServerResponse(things, http.StatusOK, w)
}

func (lr *ThingRouter) GetThing(params martini.Params, w http.ResponseWriter, thingModel *homecloud.ThingModel) {

	thing, err := thingModel.Fetch(params["id"])

	log.Infof(spew.Sprintf("thing: %v", thing))

	if err == homecloud.RecordNotFound {
		WriteServerErrorResponse(fmt.Sprintf("Unknown thing id: %s", params["id"]), http.StatusNotFound, w)
		return
	}

	if err != nil {
		WriteServerErrorResponse("Unable to retrieve thing", http.StatusInternalServerError, w)
		return
	}

	WriteServerResponse(thing, http.StatusOK, w)
}

func (lr *ThingRouter) PutThing(params martini.Params, r *http.Request, w http.ResponseWriter, thingModel *homecloud.ThingModel) {

	var thing *model.Thing

	err := json.NewDecoder(r.Body).Decode(&thing)

	if err != nil {
		WriteServerErrorResponse("Unable to parse body", http.StatusInternalServerError, w)
		return
	}

	err = thingModel.Update(params["id"], thing)

	if err != nil {
		WriteServerErrorResponse("Unable to update thing", http.StatusInternalServerError, w)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (lr *ThingRouter) PutThingLocation(params martini.Params, r *http.Request, w http.ResponseWriter, thingModel *homecloud.ThingModel) {

	thing, err := thingModel.Fetch(params["id"])

	log.Infof(spew.Sprintf("thing: %v", thing))

	if err == homecloud.RecordNotFound {
		WriteServerErrorResponse(fmt.Sprintf("Unknown thing id: %s", params["id"]), http.StatusNotFound, w)
		return
	}

	if err != nil {
		WriteServerErrorResponse("Unable to retrieve thing", http.StatusInternalServerError, w)
		return
	}

	// get the request body
	body, err := GetJsonPayload(r)

	if err != nil {
		WriteServerErrorResponse("Unable to parse body", http.StatusInternalServerError, w)
		return
	}

	roomID := body["id"].(string)

	// not a big fan of this magic
	if roomID == "" {
		err = thingModel.SetLocation(params["id"], nil)
	} else {
		err = thingModel.SetLocation(params["id"], &roomID)
	}

	if err != nil {
		WriteServerErrorResponse("Unable to save thing location", http.StatusInternalServerError, w)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (lr *ThingRouter) DeleteThing(params martini.Params, w http.ResponseWriter, thingModel *homecloud.ThingModel) {

	err := thingModel.Delete(params["id"])

	if err == homecloud.RecordNotFound {
		WriteServerErrorResponse(fmt.Sprintf("Unknown thing id: %s", params["id"]), http.StatusNotFound, w)
		return
	}

	if err != nil {
		WriteServerErrorResponse("Unable to delete thing", http.StatusInternalServerError, w)
		return
	}

	w.WriteHeader(http.StatusOK) // TODO: talk to theo about this response.
}
