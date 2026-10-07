package service

import (
	"context"
	"siakad-uts/app/model"
	"siakad-uts/app/repository"
	"siakad-uts/helper"
)

type CourseService struct{ Courses repository.CourseRepository }

func (s CourseService) List(ctx context.Context, q model.CourseQuery) ([]model.Course, error) {
	v, e := s.Courses.ListCourses(ctx, q)
	if e != nil {
		return nil, helper.Internal(e)
	}
	return v, nil
}
