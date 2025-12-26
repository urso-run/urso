<img src="./assets/urso-logo.png" width="200" alt="Urso Logo">

# Urso
Sync agent for Github Actions runners on Mac OS X

# Urso-run

Machine creation API (only once per machine/node)
POST /api/machine
Authorization: Bearer {JWT}

=> {id: uuid, token: uuid}
Request JWT from dashboard -- this is urso’s org token with 1h expiration, can be used to register as many machines as you want
Response id and token to be stored on the machine in some .urso folder
both id and token is required to talk to urso’s API
id -- in the path
token -- as authorization bearer

Machine APIs (on reconcile -- gets machine info and its runners configuration)
GET /api/machine/:id
Authorization: Bearer {token}

=> 
{runners: [ {name: string, group: string, work_dir: string, labels: []string} ]}
Request id/token from the runner creation

GET /api/machine/:id/registration-token
GET /api/machine/:id/remove-token
Authorization: Bearer {token}

=> 
{token: string}
Request id/token from the runner creation


# Commercial Use

For use within a business or commercial environment, a commercial license is required. Please contact us for details.
