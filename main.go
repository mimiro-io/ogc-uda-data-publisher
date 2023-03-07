package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

type Datahub struct {
	Url      string
	Datasets []*Dataset
}

type Dataset struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	RemoteDataset     string `json:"remoteName"`
	StripPropertyUrls bool   `json:"stripPropertyUrls"`
}

var RemoteDatahub *Datahub

func main() {
	RemoteDatahub = &Datahub{}
	RemoteDatahub.Datasets = make([]*Dataset, 0)

	loadConfig()

	e := echo.New()
	e.GET("/datasets", getDatasets)
	e.GET("/datasets/:dataset", getDataset)
	e.GET("/datasets/:dataset/changes", getChanges)
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Running")
	})
	e.Logger.Fatal(e.Start(":9042"))
}

func loadConfig() {
	// load json from config.json
	fileContent, err := os.Open("config.json")
	if err != nil {
		panic(err)
	}
	defer fileContent.Close()

	byteResult, _ := ioutil.ReadAll(fileContent)

	var res map[string]interface{}
	json.Unmarshal([]byte(byteResult), &res)

	RemoteDatahub.Url = res["uda"].(string)
	RemoteDatahub.Datasets = make([]*Dataset, 0)
	for _, ds := range res["datasets"].([]interface{}) {
		dsmap := ds.(map[string]interface{})
		newDataset := &Dataset{Name: dsmap["name"].(string), Type: dsmap["type"].(string), RemoteDataset: dsmap["remoteName"].(string)}
		if dsmap["stripPropertyUrls"] != nil {
			newDataset.StripPropertyUrls = dsmap["stripPropertyUrls"].(bool)
		}
		RemoteDatahub.Datasets = append(RemoteDatahub.Datasets, newDataset)
	}
}

func lookupDataset(name string) *Dataset {
	for _, ds := range RemoteDatahub.Datasets {
		if ds.Name == name {
			return ds
		}
	}
	return nil
}

// return the datasets in the remote datahub as json
func getDatasets(c echo.Context) error {
	return c.JSON(http.StatusOK, RemoteDatahub.Datasets)
}

// get the dataset with the given name
func getDataset(c echo.Context) error {
	name := c.Param("dataset")

	ds := lookupDataset(name)
	if ds == nil {
		return c.NoContent(http.StatusNotFound)
	}

	return c.JSON(http.StatusOK, ds)
}

func getChanges(c echo.Context) error {
	// get from remote dataset
	name := c.Param("dataset")
	since := c.QueryParam("since")

	// lookup the dataset
	ds := lookupDataset(name)

	if ds == nil {
		return c.NoContent(http.StatusNotFound)
	}

	// get the changes from the remote datahub
	requestURL := RemoteDatahub.Url + "/datasets/" + ds.RemoteDataset + "/changes"

	if since != "" {
		requestURL += "?since=" + since + "&limit=1000&latestOnly=true"
	} else {
		requestURL += "?latestOnly=true&limit=1000"
	}
	res, err := http.Get(requestURL)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	} else {
		if res.StatusCode == http.StatusOK {
			defer res.Body.Close()
			parser := NewEntityParser()
			ec, err := parser.Parse(res.Body)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if ds.Type == "features" {
				geoJson, _ := convertToFeatures(ec, ds.StripPropertyUrls)
				return c.JSON(http.StatusOK, geoJson)
			} else if ds.Type == "featureCollections" {
				geoJson, _ := convertToFeatureCollections(ec)
				return c.JSON(http.StatusOK, geoJson)
			}
			return c.NoContent(http.StatusBadRequest)
		} else {
			return c.NoContent(res.StatusCode)
		}
	}
}

func convertToFeatureCollections(ec *EntityCollection) ([]*FeatureCollection, error) {
	return nil, nil
}

// func to convert from UDA to GeoJSON
func convertToFeatures(ec *EntityCollection, stripPropertyUrls bool) ([]any, error) {
	features := make([]any, 0)

	// add empty context object
	context := make(map[string]interface{})
	context["id"] = "@context"
	features = append(features, context)

	// add all features
	for _, e := range ec.Entities {
		f := &Feature{}
		f.Id = e.ID
		f.IsDeleted = e.IsDeleted
		f.Type = "Feature"

		f.Geometry, _ = makeGeomentryFromEntity(e)

		// map all entity properties to the geojson properties
		f.Properties = make(map[string]interface{})
		for k, v := range e.Properties {
			k = stripUrl(k)
			f.Properties[k] = v
		}
		features = append(features, f)
	}

	// continuation token
	if ec.Continuation.Token != "" {
		contination := make(map[string]interface{})
		contination["id"] = "@continuation"
		contination["token"] = ec.Continuation.Token
		features = append(features, contination)
	}

	return features, nil
}

func stripUrl(url string) string {
	if strings.Contains(url, "#") {
		return strings.Split(url, "#")[1]
	} else if strings.Contains(url, "/") {
		// find last slash in url
		lastSlash := strings.LastIndex(url, "/")
		return url[lastSlash+1:]
	}
	return url
}

func makeGeomentryFromEntity(e *Entity) (*Geometry, error) {
	geotypePredicate := "http://data.mimiro.io/models/flatgeo/geotype"
	pointType := "http://data.mimiro.io/models/flatgeo/Point"
	polygonType := "http://data.mimiro.io/models/flatgeo/Polygon"

	g := &Geometry{}
	g.Coordinates = make([]interface{}, 0)

	geometryType, _ := e.getReferenceValue(geotypePredicate)
	if geometryType == pointType {
		g.Type = "Point"
		coords := e.Properties["http://data.mimiro.io/models/flatgeo/coordinates"]
		for _, c := range coords.([]interface{}) {
			g.Coordinates = append(g.Coordinates, c)
		}
	} else if geometryType == polygonType {
		g.Type = "Polygon"
		coords := e.Properties["http://data.mimiro.io/models/flatgeo/coordinates"].([]interface{})
		// iterate over list of coords and create a geojson pologon where there are 2 elements in each list
		coordList := make([]interface{}, 0)
		for i := 0; i < len(coords); i += 2 {
			coordList = append(coordList, coords[i])
			coordList = append(coordList, coords[i+1])
			g.Coordinates = append(g.Coordinates, coordList)

			// reset the coordList
			coordList = make([]interface{}, 0)
		}
	} else {
		return nil, errors.New("unknown geometry type: " + geometryType)
	}

	return g, nil
}

type FeatureCollection struct {
	Id          string    `json:"id"`
	Type        string    `json:"type"`
	BoundingBox []float64 `json:"bbox"`
	AssetType   string    `json:"assetType,omitempty"`
	AssetLink   string    `json:"assetLink,omitempty"`
	IsDeleted   bool      `json:"isDeleted,omitempty"`
}

type Feature struct {
	Id          string                 `json:"id"`
	Type        string                 `json:"type"`
	BoundingBox []float64              `json:"bbox,omitempty"`
	Geometry    *Geometry              `json:"geometry"`
	Properties  map[string]interface{} `json:"properties"`
	AssetType   string                 `json:"assetType,omitempty"`
	AssetLink   string                 `json:"assetLink,omitempty"`
	IsDeleted   bool                   `json:"isDeleted"`
}

type Geometry struct {
	Type        string        `json:"type"`
	Coordinates []interface{} `json:"coordinates"`
}
