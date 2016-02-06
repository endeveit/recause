#!/usr/bin/env bash

# Add elasticsearch repository
wget -qO - https://packages.elastic.co/GPG-KEY-elasticsearch | sudo apt-key add -
echo "deb http://packages.elastic.co/elasticsearch/2.x/debian stable main" | sudo tee -a /etc/apt/sources.list.d/elasticsearch-2.x.list

sudo apt-get update
sudo apt-get upgrade
sudo apt-get dist-upgrade

# Install openjdk-7-jdk and elasticsearch
sudo apt-get -y install openjdk-7-jdk elasticsearch
sudo update-rc.d elasticsearch defaults 95 10
sudo systemctl daemon-reload
sudo systemctl enable elasticsearch.service

# To access "localhost:9200" from the host
echo "network.host: 0.0.0.0" | sudo tee -a /etc/elasticsearch/elasticsearch.yml

# Enable CORS (to be able to use Sense)
echo "http.cors.enabled: true" | sudo tee -a /etc/elasticsearch/elasticsearch.yml
echo "http.cors.allow-origin: /https?:\/\/localhost(:[0-9]+)?/" | sudo tee -a /etc/elasticsearch/elasticsearch.yml

# Enable scripting
echo "script.inline: on" | sudo tee -a /etc/elasticsearch/elasticsearch.yml

# Start elasticsearch
sudo service elasticsearch restart

sleep 2
while ! grep -m1 'indices into cluster_state' < /var/log/elasticsearch/elasticsearch.log; do
    sleep 2
done

# Setup analyzers and mapping
curl -XPUT 'http://localhost:9200/recause/?pretty'
curl -XPOST 'http://localhost:9200/recause/_close?pretty'
curl -XPUT 'http://localhost:9200/recause/_settings?pretty' -d '
{
  "analysis":{
    "analyzer":{
      "string_ci_analyzer":{
        "type":"custom",
        "tokenizer":"keyword",
        "filter":[
          "filter_lowercase"
        ]
      },
      "text_analyzer":{
        "type":"custom",
        "tokenizer":"whitespace",
        "filter":[
          "filter_delimiter",
          "filter_lowercase"
        ]
      }
    },
    "filter":{
      "filter_delimiter":{
        "type": "word_delimiter",
        "split_on_numerics": false,
        "split_on_case_change": false
      },
      "filter_lowercase":{
        "type":"lowercase"
      }
    }
  }
}
'
curl -XPOST 'http://localhost:9200/recause/_open?pretty'
curl -XPUT 'http://localhost:9200/recause/message/_mapping?pretty' -d '
{
  "message": {
    "_all": {
      "enabled": true
    },
    "_source": {
      "enabled": true
    },
    "_ttl": {
      "enabled": true
    },
    "properties": {
      "version": {
        "type": "string",
        "store": true,
        "index": "analyzed",
        "analyzer": "string_ci_analyzer"
      },
      "host": {
        "type": "string",
        "store": true,
        "index": "analyzed",
        "analyzer": "string_ci_analyzer"
      },
      "file": {
        "type": "string",
        "store": true,
        "index": "analyzed",
        "analyzer": "string_ci_analyzer"
      },
      "facility": {
        "type": "string",
        "store": true,
        "index": "analyzed",
        "analyzer": "string_ci_analyzer"
      },
      "short_message": {
        "type": "string",
        "store": true,
        "index": "analyzed",
        "analyzer": "text_analyzer"
      },
      "full_message": {
        "type": "string",
        "store": true,
        "index": "analyzed",
        "analyzer": "text_analyzer"
      },
      "timestamp": {
        "type": "date",
        "store": true,
        "index": "not_analyzed"
      },
      "level": {
        "type": "integer",
        "store": true,
        "index": "not_analyzed"
      },
      "line": {
        "type": "integer",
        "store": true,
        "index": "not_analyzed"
      }
    }
  }
}
'
