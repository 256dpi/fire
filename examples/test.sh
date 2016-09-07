#!/usr/bin/env bash

echo "==> List posts"

curl -s -H "Accept: application/vnd.api+json" http://0.0.0.0:4000/api/posts

echo "==> Create new post"

post='{
  "data": {
    "type": "posts",
    "attributes": {
      "title": "Hello world"
    }
  }
}'

curl -s -X "POST" -H "Accept: application/vnd.api+json" \
  -H "Content-Type: application/vnd.api+json" http://0.0.0.0:4000/api/posts \
  -d "$post"

echo "==> List posts again"

curl -s -H "Accept: application/vnd.api+json" http://0.0.0.0:4000/api/posts

#echo "==> Update post"
#
#post='{
#  "data": {
#    "type": "posts",
#    "id": "1",
#    "attributes": {
#      "title": "Very cool"
#    }
#  }
#}'
#
#curl -s -X "PATCH" -H "Accept: application/vnd.api+json" \
#  -H "Content-Type: application/vnd.api+json" http://0.0.0.0:4000/api/posts/1 \
#  -d "$post"
#
#echo "==> Show post"
#
#curl -s -H "Accept: application/vnd.api+json" http://0.0.0.0:4000/api/posts/1
#
#echo "==> Delete post"
#
#curl -s -X "DELETE" -H "Accept: application/vnd.api+json" http://0.0.0.0:4000/api/posts/1
#
#echo "==> List posts again"
#
#curl -s -H "Accept: application/vnd.api+json" http://0.0.0.0:4000/api/posts
