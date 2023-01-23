package resampler

import (
	"fmt"
	"math"
)

type Resampler struct {
	filter       []float64
	filterDelta  []float64
	precision    int
	from         int
	to           int
	timeStampIdx int
	window       *window
}

func New(highQuality bool, from int, to int) *Resampler {
	var f *filter
	if highQuality {
		f = highQualityFilter
	} else {
		f = fastQualityFilter
	}

	sampleRatio := float64(to) / float64(from)
	if sampleRatio < 1.0 {
		multiply(f.arr, sampleRatio)
	}

	return &Resampler{
		filter:      f.arr,
		filterDelta: deltaOf(f.arr),
		precision:   int(f.precision),
		from:        from,
		to:          to,
		window:      newWindow(),
	}
}

func (r *Resampler) Resample(in []float64) ([]float64, error) {
	if err := r.supply(in); err != nil {
		return nil, err
	}
	return r.read(), nil
}

func (r *Resampler) supply(buf []float64) error {
	if r.window.capacity() < len(buf) {
		return fmt.Errorf("window capacity is not enough")
	}
	for _, b := range buf {
		_ = r.window.push(b)
	}
	return nil
}

func (r *Resampler) read() []float64 {
	var ret []float64

	scale := math.Min(float64(r.to)/float64(r.from), 1.0)
	indexStep := int(scale * float64(r.precision))
	nWin := len(r.filter)

	for r.window.hasEnoughPadding() {

		var sample float64

		timestamp := r.timestamp()

		frac := scale * (timestamp - float64(r.window.cursor()))
		indexFrac := frac * float64(r.precision)
		offset := int(indexFrac)
		eta := indexFrac - float64(offset)
		iMax := min(r.window.leftPadding()+1, (nWin-offset)/indexStep)

		for i := 0; i < iMax; i++ {
			idx := offset + i*indexStep
			weight := r.filter[idx] + r.filterDelta[idx]*eta
			s, err := r.window.get(-i)
			// TODO: handle error, panic 은 임시, 코드 문제가 아니라면 일어나지 않는 에러
			if err != nil {
				panic(err)
			}
			sample += weight * s
		}

		frac = scale - frac
		indexFrac = frac * float64(r.precision)
		offset = int(indexFrac)
		eta = indexFrac - float64(offset)
		kMax := min(r.window.rightPadding()-1, (nWin-offset)/indexStep)

		for k := 0; k < kMax; k++ {
			idx := offset + k*indexStep
			weight := r.filter[idx] + r.filterDelta[idx]*eta
			s, err := r.window.get(k + 1)
			// TODO: handle error, panic 은 임시, 코드 문제가 아니라면 일어나지 않는 에러
			if err != nil {
				panic(err)
			}
			sample += weight * s
		}

		ret = append(ret, sample)

		beforeCur := int(timestamp)
		r.timeStampIdx++
		afterCur := int(r.timestamp())
		// TODO: handle error, panic 은 임시, 코드 문제가 아니라면 일어나지 않는 에러
		if err := r.window.increaseCursor(afterCur - beforeCur); err != nil {
			panic(err)
		}
	}
	return ret
}

func (r *Resampler) timestamp() float64 {
	return float64(r.timeStampIdx) * float64(r.from) / float64(r.to)
}
