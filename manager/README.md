# Manager

Accepts and manages customer requests.

## API

### Hash Crack Request

- **Method:** POST
- **Path:** `/api/hash/crack`
- **Request Body:**
```json
{
  "hash": "e2fc714c4727ee9395f324cd2e7f331f",
  "maxLength": 4
}
```

- **Response:**
```json
{
  "requestId": "730a04e6-4de9-41f9-9d5b-53b88b17afac"
}
```

### Task status

- **Method:** GET
- **Path:** `/api/hash/status?requestId=`
- **Response:**
```json
{
  "status": "READY",
  "data": ["abcd"]
}

```
## Configuration

```kdl
log-level "debug"
api-server-addr "0.0.0.0:8080"
worker-server-addr "0.0.0.0:9090"
dispatcher {
    request-queue-size 100
    dispatch-timeout "10s"
    request-timeout "2m"
    reconnect-timeout "1s"
    health-timeout "10s"
}
consul {
    address "consul:8500"
    health {
        interval "30s"
        timeout "5s"
        http "/api/health"
    }
}
amqp {
    uri "amqp://guest:guest@rabbitmq:5672/"
    username "guest"
    password "guest"
    reconnect-timeout "3s"
    publisher {
        exchange "exchange.crack.request"
        routing-key "crack.request"
    }
    consumer {
        queue "queue.crack.response"
    }
}
mongo {
    uri "mongodb://admin:secret@mongo-primary:27017,mongo-secondary1:27017,mongo-secondary2:27017/?replicaSet=rs0&authSource=admin"
    username "admin"
    password "secret"
    database "requests"
}
```

