# Copyright 2009 Dimiter Stanev, malkia@gmail.com. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

all: install

include $(GOROOT)/src/Make.$(GOARCH)

TARG    = go
GOFILES = $(TARG).go

include $(GOROOT)/src/Make.cmd
