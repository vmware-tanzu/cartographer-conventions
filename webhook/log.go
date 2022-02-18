/*
Copyright 2020 VMware Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"log"

	"github.com/go-logr/logr"
)

type LoggerKey struct{}

func loggerFromContext(ctx context.Context) *logr.Logger {
	lv := ctx.Value(LoggerKey{})
	if logger, ok := lv.(*logr.Logger); ok {
		return logger
	}
	return nil
}

func logInfo(logger *logr.Logger, msg string, keysAndValues ...interface{}) {
	if logger != nil {
		logger.Info(msg, keysAndValues...)
	} else {
		v := make([]interface{}, len(keysAndValues)+1)
		v[0] = msg
		v = append(v, keysAndValues...)
		log.Println(v...)
	}
}

func logError(logger *logr.Logger, err error, msg string, keysAndValues ...interface{}) {
	if logger != nil {
		logger.Error(err, msg, keysAndValues...)
	} else {
		v := make([]interface{}, len(keysAndValues)+2)
		v[0] = msg
		v[1] = err
		v = append(v, keysAndValues...)
		log.Println(v...)
	}
}
