#!/bin/bash

go build -o BUILD/linux-x64/needle .
ftr pack . -C needle
ftr up needle*.sqar JFtR/needle
rm needle*.sqar

ftr query JFtR/needle
