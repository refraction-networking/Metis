# -*- coding: utf-8 -*-
#!/usr/bin/python

from flask import Flask, request
from flask_restful import Resource, Api, reqparse
from json import dumps

app = Flask(__name__)
api = Api(app)

parser = reqparse.RequestParser()

class BlockedList(Resource):
    def get(self):
        print("Request for blocked list made by client: ")
        return [{'Domain':'Earth'}]
    
    def post(self):
        print("Client sent RAPPOR things to us: ")
        args = parser.parse_args()
        print(args)
        return '', 200

api.add_resource(BlockedList, "/blocked")
print("Starting Metis server...")
app.run(debug=True)
    