package imu

import (
	"fmt"
	"time"

	"github.com/streamingfast/imu-controller/device/iim42652"
)

type RawFeed struct {
	imu      *iim42652.IIM42652
	handlers []RawFeedHandler
}

func NewRawFeed(imu *iim42652.IIM42652, handlers ...RawFeedHandler) *RawFeed {
	return &RawFeed{
		imu:      imu,
		handlers: handlers,
	}
}

type RawFeedHandler func(acceleration *Acceleration, angularRate *iim42652.AngularRate, temperature iim42652.Temperature) error

//TODO: add FileWatcherEventFeed
// and have imu raw subscribe to it
// have 1 jpg that we will keep on reusing the same image
// then create the frameKms and the gz files with the same image over and over again
// inspire on the file watcher in the hdc-debugger

func (f *RawFeed) Run(axisMap *iim42652.AxisMap) error {
	fmt.Println("Run imu raw feed")
	for {
		time.Sleep(25 * time.Millisecond)
		acceleration, err := f.imu.GetAcceleration()
		if err != nil {
			return fmt.Errorf("getting acceleration: %w", err)
		}

		angularRate, err := f.imu.GetGyroscopeData()
		if err != nil {
			return fmt.Errorf("getting angular rate: %w", err)
		}

		temperature, err := f.imu.GetTemperature()
		if err != nil {
			return fmt.Errorf("getting temperature: %w", err)
		}

		for _, handler := range f.handlers {
			err := handler(
				NewAcceleration(axisMap.X(acceleration), axisMap.Y(acceleration), axisMap.Z(acceleration), acceleration.TotalMagnitude, time.Now()),
				angularRate,
				temperature,
			)
			if err != nil {
				return fmt.Errorf("calling handler: %w", err)
			}
		}
		if angularRate.X < -2000.0 {
			fmt.Println("Resetting imu because angular rate is too high:", angularRate.X)
			err := f.imu.Init()
			if err != nil {
				return fmt.Errorf("initializing IMU: %w", err)
			}

			//err := f.imu.ResetSignalPath()
			//if err != nil {
			//	return fmt.Errorf("resetting signal path: %w", err)
			//}
		}
	}
}
