# Readme

## Usage

`SEED_ON_STARTUP=true docker compose up --build` will run the entire project, with MongoDB and Redis, and seed the two pieces of equipment from the instructions
into the database. This means the equipment collection will contain the following values:
```json
[
  {
    "ID": "2e85d43d-dd9b-4e8d-b2ce-97b8d7d69d49",
    "Name": "AirCat - Drill - 4337",
    "VibrationMagnitude": 2.1
  },
  {
    "ID": "36603447-2f30-41b1-a908-526c0b6f1755",
    "Name": "JCB - Hydraulic Breaker - CEJCBHM25",
    "VibrationMagnitude": 4.0
  }
]
```
It also seeds a few users:
```json
[
  {
    "ID": "713be58e-0d79-4df2-a85c-9f44ca513a7d",
    "Name": "Bobby Tables"
  },
  {
    "ID": "b3a0eddc-e20d-453b-893e-36794a1daffe",
    "Name": "Ada Lovelace"
  },
  {
    "ID": "78776e50-0e1a-4282-ba37-83d54c1b4795",
    "Name": "Grace Hopper"
  }
]
```

The repository includes an `example-requests.http` [.http file](https://http-files.org) with some sample API requests.

## Architecture
The code is split into three layers, each only depending inward:
* `internal/domain` — entities (`User`, `EquipmentItem`, `Exposure`, `ExposureSummary`) and the repository/publisher interfaces they're accessed through.
This is where the A(8)/Points formulas and the linear-sum-vs-quadrature aggregation logic live (though the aggregation is duplicated in `internal/infra`).
* `internal/app` — `ExposureService`, which orchestrates the domain and ports: looking up users/equipment, persisting exposures, resolving the nested `user` field, and publishing events.
* `internal/http` and `internal/infra` — adapters. `internal/http` implements the spec's routes over `ExposureService`; `internal/infra/mongodb` and `internal/infra/redis` implement the domain's repository/publisher interfaces.

`internal/app` and `internal/domain` have no dependency on MongoDB, Redis, or `net/http` — they're tested with in-memory fakes, not a live stack.

The user's exposure summary is computed via a MongoDB aggregation pipeline. This does mean the data layer encodes some knowledge of the application's business logic, but I think that is justifiable for the scalability in a real-world system.

## Events
The service publishes two different types of events, these events are not actually used but serve as examples of events the system might want to publish:
* `exposure.recorded` event when a new exposure is added
* `exposure.orphaned` when an exposure is detected that does not have an associated user

The most obvious omission is an `exposure.exceeded` event, to notify when a user has exceeded their exposure limit.

## Divergences from spec
There are a couple of places where this implementation diverges from the spec:

* The spec says /users/{userId}/exposure-summary takes `starting_at` and `ending_at` with format `date`, but I believe this should be `date-time` per the https://swagger.io/docs/specification/v3_0/data-models/data-types/[OpenAPI spec]
* `GET /exposure/{exposureId}` listed a 201 response status code, but nothing is being created for this route. In this implementation, I have returned 200.
In a real-world scenario, I would want to confirm this change is appropriate. If an external system is calling the API, it's possible the 201 code is required.


## Caveats/desired extensions
- `GET /exposure` returns all exposures. This matches the API spec, but is probably not desirable. The endpoint should be paged.
- Originally, I expected to implement an inbox pattern and MongoDB is a good fit for that model.
For the design as-implemented, it would probably have been simpler to use a relational database.
- Equipment data is duplicated into the exposure collection. I think this is probably unnecessary, though it could be desirable
to preserve equipment specs at-time-of-exposure.  
- The repository layer is insufficiently tested.
I'd like to either mock the MongoDB driver or have some automatic end-to-end tests with appropriate setup and teardown.
