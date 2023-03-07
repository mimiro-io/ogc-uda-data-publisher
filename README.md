# OGC UDA Data Publisher
A slim service that exposes UDA data endpoint as Open Ocean Data Sharing endpoint. The resulting stream of data is either a stream of GeoJSON Features or a strema of GeoJSON FeatureCollections.

# Installation

This assume that you have a working UDA endpoint from which you will consume and then publish the data. The  UDA endpoint can be a MIMIRO datahub dataset or it can talk directly to any UDA compliant endpoint that supports the changes endpoint.

The docker image can be built as follows:

```bash 
docker build -t mimiro/ogc-data-publisher .
```

The service can be run as a docker container. The following command will run the service and expose it on port 9042.

The configuration file is mounted as a volume. The configuration file is described in the next section.
    
```bash
docker run -p 9042:9042 -v /path/to/config.json:/root/config.json mimiro/ogc-data-publisher
```

# Configuration

The configuration is done via a config file. The config file is a JSON file that contains the following structure:

```json
{
    "uda" : "http://localhost:4242",
    "datasets" : [
        {
            "name" : "wod",
            "type" : "featurecollections",
            "remoteName" : "wod"
        },
        {
            "name" : "jellyfish",
            "type" : "features",
            "remoteName" : "ocean.jellyfish",
            "stripPropertyUrls" : true
        }
    ]
}
```

The `uda` property is the URL to the UDA endpoint. The `datasets` property is an array of datasets that you want to expose. Each dataset has the following properties:

* `name` - the name of the dataset. This is the name that will be used in the URL to access the dataset.
* `type` - the type of the dataset. This can be either `features` or `featurecollections`. The `features` type will expose a stream of GeoJSON Features. The `featurecollections` type will expose a stream of GeoJSON FeatureCollections.
* `remoteName` - the name of the dataset in the UDA endpoint. This is the name that will be used in the UDA endpoint to access the dataset.
* `stripPropertyUrls` - a boolean value that indicates whether the property URLs should be stripped from the data. This is useful if you want to expose the data to a client that does not support property URLs.

# Data Shape from UDA endpoint

This service assumes that the data in the UDA endpoint is in the following shape for features. The required properties are:

```
http://data.mimiro.io/models/flatgeo/geotype
http://data.mimiro.io/models/flatgeo/coordinates
http://data.mimiro.io/models/flatgeo/bbox
```

```json
{
  "id": "@context",
  "namespaces": {
    "ns0": "http://data.mimiro.io/core/dataset/",
    "ns1": "http://data.mimiro.io/core/",
    "ns2": "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
    "ns3": "http://ocean.data.example.org/jellyfish/reports/",
    "ns4": "http://data.mimiro.io/models/flatgeo/",
    "ns5": "http://ocean.data.example.org/jellyfish/"
  }
}
,{
  "id": "ns3:116",
  "recorded": 1678133597087688448,
  "deleted": false,
  "refs": {
    "ns2:type": "ns4:Feature",
    "ns4:geotype": "ns4:Point"
  },
  "props": {
    "ns4:bbox": [100, 456.34, 600],
    "ns4:coordinates": [32.355448, 34.790955],
    "ns5:Activity": "Yacht",
    "ns5:Area": "Center",
    "ns5:Date": "01/07/2011",
    "ns5:Distance from coast": "200 meters-1 mile",
    "ns5:Gold": "1",
    "ns5:Jellies on the beach": "1",
    "ns5:Lat.": "32.355448",
    "ns5:Location": "Had-Jsr",
    "ns5:Long.": "34.790955",
    "ns5:Month": "7",
    "ns5:Quantity": "50",
    "ns5:Report": "Offshore",
    "ns5:Report ID": "116",
    "ns5:Season": "Summer",
    "ns5:Size": "Oct-30",
    "ns5:Species": "Rhopilema nomadica",
    "ns5:Stinging Water": "1",
    "ns5:Time of submition": "9",
    "ns5:Website version": "1.0",
    "ns5:Week": "27",
    "ns5:Year": "2011",
    "ns5:Zone #": "9"
  }
}
```

and like this for feature collections:

```json

```

