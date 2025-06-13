#!/bin/sh

uv tool install --python=$(which python$(cat .python-version)) .
