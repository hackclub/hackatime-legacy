package services

import (
	"testing"
	"time"

	"github.com/hackclub/hackatime/mocks"
	"github.com/hackclub/hackatime/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type HousekeepingServiceTestSuite struct {
	suite.Suite
	TestUsers        []*models.User
	UserService      *mocks.UserServiceMock
	HeartbeatService *mocks.HeartbeatServiceMock
	SummaryService   *mocks.SummaryServiceMock
}

type dataDumpServiceMock struct {
	mock.Mock
}

func (m *dataDumpServiceMock) GetByUser(userID string) ([]*models.DataDump, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.DataDump), args.Error(1)
}

func (m *dataDumpServiceMock) Create(user *models.User, dumpType string) (*models.DataDump, error) {
	args := m.Called(user, dumpType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DataDump), args.Error(1)
}

func (m *dataDumpServiceMock) MarkStuckDumps() error {
	args := m.Called()
	return args.Error(0)
}

func (m *dataDumpServiceMock) CleanupExpired() error {
	args := m.Called()
	return args.Error(0)
}

func (m *dataDumpServiceMock) DeleteByUser(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

func (suite *HousekeepingServiceTestSuite) SetupSuite() {
	suite.TestUsers = []*models.User{
		{ID: "testuser01", LastLoggedInAt: models.CustomTime(time.Now().AddDate(0, -16, 0)), HasData: false},
		{ID: "testuser02", LastLoggedInAt: models.CustomTime(time.Now().AddDate(0, -16, 0)), HasData: true},
		{ID: "testuser03", LastLoggedInAt: models.CustomTime(time.Now().AddDate(0, -1, 0)), HasData: false},
	}
}

func (suite *HousekeepingServiceTestSuite) BeforeTest(suiteName, testName string) {
	suite.UserService = new(mocks.UserServiceMock)
	suite.HeartbeatService = new(mocks.HeartbeatServiceMock)
	suite.SummaryService = new(mocks.SummaryServiceMock)
}

func TestHouseKeepingServiceTestSuite(t *testing.T) {
	suite.Run(t, new(HousekeepingServiceTestSuite))
}

func (suite *HousekeepingServiceTestSuite) TestHousekeepingService_CleanInactiveUsers() {
	sut := NewHousekeepingService(suite.UserService, suite.HeartbeatService, suite.SummaryService, nil)

	suite.UserService.On("GetAll").Return(suite.TestUsers, nil)
	suite.UserService.On("Delete", suite.TestUsers[0]).Return(nil)

	err := sut.CleanInactiveUsers(time.Now().AddDate(0, -12, 0))

	assert.Nil(suite.T(), err)
	suite.UserService.AssertNumberOfCalls(suite.T(), "GetAll", 1)
	suite.UserService.AssertNumberOfCalls(suite.T(), "Delete", 1)
	suite.UserService.AssertCalled(suite.T(), "Delete", suite.TestUsers[0])
}

func (suite *HousekeepingServiceTestSuite) TestHousekeepingService_CleanUserDataBefore_CleansDataDumps() {
	dataDumpService := new(dataDumpServiceMock)
	user := &models.User{ID: "cleanup-user"}
	before := time.Now().AddDate(0, -1, 0)

	sut := NewHousekeepingService(suite.UserService, suite.HeartbeatService, suite.SummaryService, dataDumpService)

	dataDumpService.On("DeleteByUser", user.ID).Return(nil)
	suite.HeartbeatService.On("DeleteByUserBefore", user, before).Return(nil)
	suite.SummaryService.On("DeleteByUserBefore", user.ID, before).Return(nil)

	err := sut.CleanUserDataBefore(user, before)

	assert.NoError(suite.T(), err)
	dataDumpService.AssertCalled(suite.T(), "DeleteByUser", user.ID)
	suite.HeartbeatService.AssertCalled(suite.T(), "DeleteByUserBefore", user, before)
	suite.SummaryService.AssertCalled(suite.T(), "DeleteByUserBefore", user.ID, before)
}
