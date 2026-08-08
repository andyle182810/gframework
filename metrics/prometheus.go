package metrics

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

func RegisterCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec {
	collector := prometheus.NewCounterVec(opts, labelNames)

	if err := prometheus.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.CounterVec)
			if ok {
				return existing
			}
		}

		panic(err)
	}

	return collector
}

func RegisterGaugeVec(opts prometheus.GaugeOpts, labelNames []string) *prometheus.GaugeVec {
	collector := prometheus.NewGaugeVec(opts, labelNames)

	if err := prometheus.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.GaugeVec)
			if ok {
				return existing
			}
		}

		panic(err)
	}

	return collector
}

func RegisterHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec {
	collector := prometheus.NewHistogramVec(opts, labelNames)

	if err := prometheus.Register(collector); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			existing, ok := alreadyRegistered.ExistingCollector.(*prometheus.HistogramVec)
			if ok {
				return existing
			}
		}

		panic(err)
	}

	return collector
}

func CounterOpts(namespace, subsystem, name, help string) prometheus.CounterOpts {
	return prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	}
}

func GaugeOpts(namespace, subsystem, name, help string) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: nil,
	}
}

func HistogramOpts(namespace, subsystem, name, help string, buckets []float64) prometheus.HistogramOpts {
	return prometheus.HistogramOpts{
		Namespace:                       namespace,
		Subsystem:                       subsystem,
		Name:                            name,
		Help:                            help,
		ConstLabels:                     nil,
		Buckets:                         buckets,
		NativeHistogramBucketFactor:     0,
		NativeHistogramZeroThreshold:    0,
		NativeHistogramMaxBucketNumber:  0,
		NativeHistogramMinResetDuration: 0,
		NativeHistogramMaxZeroThreshold: 0,
		NativeHistogramMaxExemplars:     0,
		NativeHistogramExemplarTTL:      0,
	}
}
