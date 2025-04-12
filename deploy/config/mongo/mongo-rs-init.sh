#!/bin/bash
echo "Initializing replicaset"
mongosh --host mongo-primary:27017 -u $MONGO_USERNAME -p $MONGO_PASSWORD --authenticationDatabase admin <<EOF
  var cfg = {
    "_id": "rs0",
    "members": [
      { _id: 0, host: "mongo-primary:27017" },
      { _id: 1, host: "mongo-secondary1:27017" },
      { _id: 2, host: "mongo-secondary2:27017" }
    ]
  };
  rs.initiate(cfg);
  print("Replica set initialized.");
EOF
echo "Done"