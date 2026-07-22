#!/bin/bash

# takes arguments
# $1 -- theme

rm -rf static/*
cp -r themes/$1/* static