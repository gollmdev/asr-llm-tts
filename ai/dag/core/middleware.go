package dag

import "log"

func TraceMiddleware() EmitMiddleware {
	return func(next EmitFunc) EmitFunc {
		return func(ev *Event) {
			// tracer.Record(nodeID, ev.Type)
			next(ev)
		}
	}
}

type Logger string

func LoggingMiddleware() EmitMiddleware {
	return func(next EmitFunc) EmitFunc {
		return func(ev *Event) {
			// logger.Infof("node=%s emit type=%s", nodeID, ev.Type)
			log.Printf("node=%s emit type=%s", "nodeID", ev.Type)
			next(ev)
		}
	}
}

// func RecoverMiddleware(logger Logger) EmitMiddleware {
//     return func(nodeID string, next EmitFunc) EmitFunc {
//         return func(ev Event) {
//             defer func() {
//                 if r := recover(); r != nil {
//                     logger.Errorf("panic in node=%s: %v", nodeID, r)
//                 }
//             }()
//             next(ev)
//         }
//     }
// }

//	func MemoryMiddleware(memory MemoryStore) EmitMiddleware {
//	    return func(nodeID string, next EmitFunc) EmitFunc {
//	        return func(ev Event) {
//	            memory.Append(nodeID, ev)
//	            next(ev)
//	        }
//	    }
//	}

// func MetricsMiddleware(metrics Metrics) EmitMiddleware {
// 	return func(nodeID string, next EmitFunc) EmitFunc {
// 		return func(ev Event) {
// 			metrics.Count("event_emit_total")
// 			next(ev)
// 		}
// 	}
// }

// func MemoryMiddleware(memory MemoryStore) EmitMiddleware {
//     return func(nodeID string, next EmitFunc) EmitFunc {
//         return func(ev Event) {
//             memory.Append(nodeID, ev)
//             next(ev)
//         }
//     }
// }
