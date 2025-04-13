# Worker

Cracking the md5 hash.

## Configuration

```kdl
log-level "debug"
server-port 8081
manager-url "manager:9090"
consul {
    address "consul:8500"
    health {
        interval "5s"
        timeout "5s"
        http "/api/health"
        deregister-timeout "10s"
    }
}
amqp {
    uri "amqp://guest:guest@rabbitmq:5672/"
    username "guest"
    password "guest"
    reconnect-timeout "3s"
    publisher {
        exchange "exchange.crack.response"
        routing-key "crack.response"
    }
    consumer {
        queue "queue.crack.request"
    }
}
```