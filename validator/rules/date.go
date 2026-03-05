package rules

import (
	"time"

	"github.com/leandroluk/gonest/validator"
)

// DateAfter validates that date is after the specified date
func DateAfter(after time.Time) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.After(after) {
			err := validator.NewFieldError(
				"",
				"date_after",
				"Date must be after the specified date",
			)
			err.WithParam("after", after)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// DateBefore validates that date is before the specified date
func DateBefore(before time.Time) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.Before(before) {
			err := validator.NewFieldError(
				"",
				"date_before",
				"Date must be before the specified date",
			)
			err.WithParam("before", before)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// DateBetween validates that date is between two dates
func DateBetween(start, end time.Time) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if value.Before(start) || value.After(end) {
			err := validator.NewFieldError(
				"",
				"date_between",
				"Date must be between the specified dates",
			)
			err.WithParam("start", start)
			err.WithParam("end", end)
			err.WithParam("actual", value)
			return err
		}
		return nil
	}
}

// DatePast validates that date is in the past
func DatePast() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.Before(time.Now()) {
			err := validator.NewFieldError(
				"",
				"date_past",
				"Date must be in the past",
			)
			err.WithParam("actual", value)
			err.WithParam("now", time.Now())
			return err
		}
		return nil
	}
}

// DateFuture validates that date is in the future
func DateFuture() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if !value.After(time.Now()) {
			err := validator.NewFieldError(
				"",
				"date_future",
				"Date must be in the future",
			)
			err.WithParam("actual", value)
			err.WithParam("now", time.Now())
			return err
		}
		return nil
	}
}

// DateToday validates that date is today
func DateToday() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		tomorrow := today.AddDate(0, 0, 1)

		if value.Before(today) || value.After(tomorrow) {
			err := validator.NewFieldError(
				"",
				"date_today",
				"Date must be today",
			)
			err.WithParam("actual", value)
			err.WithParam("today", today)
			return err
		}
		return nil
	}
}

// DateMinAge validates minimum age from birthdate
func DateMinAge(minAge int) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		now := time.Now()
		age := now.Year() - value.Year()

		// Adjust if birthday hasn't occurred this year
		if now.YearDay() < value.YearDay() {
			age--
		}

		if age < minAge {
			err := validator.NewFieldError(
				"",
				"date_min_age",
				"Age is below minimum",
			)
			err.WithParam("min_age", minAge)
			err.WithParam("actual_age", age)
			err.WithParam("birthdate", value)
			return err
		}
		return nil
	}
}

// DateMaxAge validates maximum age from birthdate
func DateMaxAge(maxAge int) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		now := time.Now()
		age := now.Year() - value.Year()

		if now.YearDay() < value.YearDay() {
			age--
		}

		if age > maxAge {
			err := validator.NewFieldError(
				"",
				"date_max_age",
				"Age is above maximum",
			)
			err.WithParam("max_age", maxAge)
			err.WithParam("actual_age", age)
			err.WithParam("birthdate", value)
			return err
		}
		return nil
	}
}

// DateWeekday validates that date is a specific weekday
func DateWeekday(weekday time.Weekday) validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		if value.Weekday() != weekday {
			err := validator.NewFieldError(
				"",
				"date_weekday",
				"Date must be on the specified weekday",
			)
			err.WithParam("expected", weekday.String())
			err.WithParam("actual", value.Weekday().String())
			return err
		}
		return nil
	}
}

// DateWeekend validates that date is on weekend (Saturday or Sunday)
func DateWeekend() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		weekday := value.Weekday()
		if weekday != time.Saturday && weekday != time.Sunday {
			err := validator.NewFieldError(
				"",
				"date_weekend",
				"Date must be on weekend",
			)
			err.WithParam("actual", weekday.String())
			return err
		}
		return nil
	}
}

// DateWeekday validates that date is on a weekday (Monday-Friday)
func DateIsWeekday() validator.Validator[time.Time] {
	return func(value time.Time) *validator.FieldError {
		weekday := value.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			err := validator.NewFieldError(
				"",
				"date_weekday",
				"Date must be on a weekday",
			)
			err.WithParam("actual", weekday.String())
			return err
		}
		return nil
	}
}
