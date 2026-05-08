#!/bin/bash

go build -o BUILD/linux-x64/needle .
ftr pack . -C needle
ftr up needle*.sqar JFtR/needle
rm needle*.sqar

ftr pack . -U needle
ftr up needle*.fsdl JFtR/needle
rm needle*.fsdl

ftr query JFtR/needle
