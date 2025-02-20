// api.go
/*
Example of API request
c.Request.Method gives you the HTTP method of the request (GET, POST, etc.).
c.Request.URL gives you the URL of the request.
c.Request.Header gives you the request headers.
c.Request.Body gives you the request body, which you can parse according to the content type (JSON, form, etc.).
*/

package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

// struct for error data (need to be JSON before return)
type ErrorData struct {
	status    int
	error_msg string
}

type UserData struct {
	Username string
	Email    string
	Pwd      string
}

type MessageData struct {
	Content string `json:"content"`
}

func updateLatest(c *gin.Context) {
	parsedCommandID := c.Query("latest")
	if parsedCommandID != "-1" {
		c.SetCookie("latestProcessedCommandId", parsedCommandID, 3600, "/", "", false, true)
	}
}

func getLatestHelper(c *gin.Context) int {
	latestProcessedCommandID, err := c.Cookie("latestProcessedCommandId")
	if err != nil || latestProcessedCommandID == "" {
		latestProcessedCommandID = "-1"
	}
	latestProcessedCommandIDInt, err := strconv.Atoi(latestProcessedCommandID)
	if err != nil || latestProcessedCommandID == "" {
		latestProcessedCommandIDInt = -1
	}
	return latestProcessedCommandIDInt
}

// api/latest
func getLatest(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"latest": getLatestHelper(c)})
}

/*
/api/register
POST
Takes data from the POST and registers a user in the db
returns: ("", 204) or ({"status": 400, "error_msg": error}, 400)
*/
func apiRegisterHandler(c *gin.Context) {

	updateLatest(c)
	latest := getLatestHelper(c)
	logMessage(fmt.Sprint(latest) + " apiRegisterHandler: registering user.")

	errorData := ErrorData{
		status:    0,
		error_msg: "",
	}

	//Check if user already exists
	userID, exists := c.Get("UserID")
	if exists {
		errorData.status = 400
		errorData.error_msg = "User already exists: " + fmt.Sprintf("%v", userID)
		c.AbortWithStatusJSON(400, errorData)
		return
	}

	if c.Request.Method == http.MethodPost {
		// Read the request body
		var registerReq UserData
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			errorData.status = 400
			errorData.error_msg = "Failed to read JSON"
			c.AbortWithStatusJSON(400, errorData)
			return
		}

		// Parse the request body from JSON
		// Unmarshal parses the JSON and stores it in a pointer (registerReq)
		if err := json.Unmarshal(body, &registerReq); err != nil {
			errorData.status = 400
			errorData.error_msg = "Failed to parse JSON"
			c.AbortWithStatusJSON(400, errorData)
			return
		}

		//Set the user data
		username := registerReq.Username
		email := registerReq.Email
		password := registerReq.Pwd

		// Get user ID
		userID, err := getUserIDByUsername(username)
		if err != nil {
			errorData.status = 400
			errorData.error_msg = "Failed to get userID"
			c.AbortWithStatusJSON(400, errorData)
			return
		}

		// Check for errors
		if username == "" {
			errorData.status = 400
			errorData.error_msg = "You have to enter a username"
			c.AbortWithStatusJSON(400, errorData.error_msg)
			return

		} else if email == "" || !strings.Contains(email, "@") {
			errorData.status = 400
			errorData.error_msg = "You have to enter a valid email address"
			c.AbortWithStatusJSON(400, errorData.error_msg)
			return

		} else if password == "" {
			errorData.status = 400
			errorData.error_msg = "You have to enter a password"
			c.AbortWithStatusJSON(400, errorData.error_msg)
			return

		} else if fmt.Sprint(userID) != "-1" {
			errorData.status = 400
			errorData.error_msg = "The username is already taken"
			c.AbortWithStatusJSON(400, errorData.error_msg)
			return

		} else {
			hash := md5.Sum([]byte(password))
			err := registerUser(username, email, hash)
			if err != nil {
				errorData.status = 400
				errorData.error_msg = "Failed to register user"
				c.AbortWithStatusJSON(400, errorData.error_msg)
				return
			}
		}

		if errorData.error_msg != "" {
			c.AbortWithStatusJSON(400, errorData.error_msg)
			return
		} else {
			c.JSON(204, "")
		}
	}
}

/*
/api/msgs
/api/msgs?no=<num>
*/
func apiMsgsHandler(c *gin.Context) {

	updateLatest(c)
	latest := getLatestHelper(c)
	logMessage(fmt.Sprint(latest) + " apiMsgsHandler: getting all messages.")

	errorData := ErrorData{
		status:    0,
		error_msg: "",
	}

	numMsgs := c.Request.Header.Get("no")
	numMsgsInt, err := strconv.Atoi(numMsgs)
	// fallback on default value
	if err != nil {
		numMsgsInt = 100
	}

	messages, err := getPublicMessages(numMsgsInt)
	if err != nil {
		errorData.status = http.StatusBadRequest
		errorData.error_msg = "Failed to fetch messages from DB"
		c.AbortWithStatusJSON(http.StatusBadRequest, errorData)
	}

	filteredMessages := filterMessages(messages)
	jsonFilteredMessages, _ := json.Marshal(filteredMessages)
	c.Header("Content-Type", "application/json")
	c.String(http.StatusOK, string(jsonFilteredMessages))
}

/*
/api/msgs/<username>
*/
func apiMsgsPerUserHandler(c *gin.Context) {

	updateLatest(c)
	latest := getLatestHelper(c)
	logMessage(fmt.Sprint(latest) + " apiMsgsPerUserHandler: getting all messages by user " + c.Param("username") + ".")

	errorData := ErrorData{
		status:    0,
		error_msg: "",
	}

	profileUserName := c.Param("username")
	userId, err := getUserIDByUsername(profileUserName)
	if userId == -1 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if c.Request.Method == http.MethodGet {
		numMsgs := c.Request.Header.Get("no")
		numMsgsInt, err := strconv.Atoi(numMsgs)
		// fallback on default value
		if err != nil {
			numMsgsInt = 100
		}

		messages, err := getUserMessages(userId, numMsgsInt)
		if err != nil {
			errorData.status = http.StatusBadRequest
			errorData.error_msg = "Failed to fetch messages from DB"
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorData)
		}

		filteredMessages := filterMessages(messages)
		jsonFilteredMessages, _ := json.Marshal(filteredMessages)
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, string(jsonFilteredMessages))

	} else if c.Request.Method == http.MethodPost {
		// Read the request body
		var messageReq MessageData
		body, err := io.ReadAll(c.Request.Body)

		if err != nil {
			errorData.status = 400
			errorData.error_msg = "Failed to read JSON"
			c.AbortWithStatusJSON(http.StatusBadRequest, errorData)
		}

		if err := json.Unmarshal(body, &messageReq); err != nil {
			errorData.status = 400
			errorData.error_msg = "Failed to parse JSON"
		}

		text := messageReq.Content
		fmt.Println(text)
		authorId, err := getUserIDByUsername(profileUserName)
		if err != nil {
			errorData.status = http.StatusBadRequest
			errorData.error_msg = "Failed to get userID"
			c.AbortWithStatusJSON(http.StatusBadRequest, errorData)
		}

		err = addMessage(text, strconv.Itoa(int(authorId)))
		if err != nil {
			errorData.status = http.StatusInternalServerError
			errorData.error_msg = "Failed to upload message"
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorData)
		}

		c.String(http.StatusNoContent, "")
	}
}

/*
GET and POST
if GET:

	return: all followers that :username follows

else if POST:

	if FOLLOW:
		make userA follow userB
		return: status code
	if UNFOLLOW:
		make userA unfollow userB
		return: status code

/api/fllws/<username>
*/
func apiFllwsHandler(c *gin.Context) {

	updateLatest(c)
	latest := getLatestHelper(c)
	logMessage(fmt.Sprint(latest) + " apiFllwsHandler: checking follow")

	errorData := ErrorData{
		status:    0,
		error_msg: "",
	}

	if c.Request.Method == http.MethodGet {
		profileUserName := c.Param("username")
		numFollr := c.Request.Header.Get("no")
		numFollrInt, err := strconv.Atoi(numFollr)
		// fallback on default value
		if err != nil {
			numFollrInt = 100
		}

		userId, err := getUserIDByUsername(profileUserName)
		if err != nil || userId == -1 {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Fetch all followers for the user
		userIdStr := strconv.FormatInt(userId, 10)
		followers, err := getFollowers(userIdStr, numFollrInt)
		if err != nil {
			errorData.status = http.StatusInternalServerError
			errorData.error_msg = "Failed to fetch followers from DB"
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorData)
		}
		// empty slice for follower usernames
		followerNames := []string{}

		// Append the usernames to the followerNames slice
		for _, follower := range followers {
			followerNames = append(followerNames, follower["username"].(string))
		}

		// Prepare response
		followersResponse := gin.H{
			"followers": followerNames,
		}

		fmt.Println(followersResponse)

		// Send JSON response of all followers
		c.JSON(200, followersResponse)

	} else if c.Request.Method == http.MethodPost {
		// POST request
		var requestBody struct {
			Follow   string `json:"follow"`
			Unfollow string `json:"unfollow"`
		}

		// Bind JSON data to requestBody
		if err := c.BindJSON(&requestBody); err != nil {
			errorData.status = http.StatusNotFound
			errorData.error_msg = "Failed to parse JSON"
			c.AbortWithStatusJSON(http.StatusNotFound, errorData)
			return
		}

		profileUserName := c.Param("username")

		// Convert profileUserName to userID
		userId, err := getUserIDByUsername(profileUserName)
		if err != nil || userId == -1 {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		userIdStr := strconv.FormatInt(userId, 10)

		if requestBody.Follow != "" {
			// Follow logic
			// Convert requestBody.Follow to profileUserID
			profileUserID, err := getUserIDByUsername(requestBody.Follow)
			if err != nil || profileUserID == -1 {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			profileUserIDStr := strconv.FormatInt(profileUserID, 10)

			// Follow the user
			if err := followUser(userIdStr, profileUserIDStr); err != nil {
				errorData.status = http.StatusNotFound
				errorData.error_msg = "Failed to follow user"
				c.AbortWithStatusJSON(http.StatusNotFound, errorData)
				return
			}

			c.JSON(http.StatusNoContent, "")
			return
		} else if requestBody.Unfollow != "" {
			// Unfollow logic
			// Convert requestBody.Unfollow to profileUserID
			profileUserID, err := getUserIDByUsername(requestBody.Unfollow)
			if err != nil || profileUserID == -1 {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			profileUserIDStr := strconv.FormatInt(profileUserID, 10)

			// Unfollow the user
			if err := unfollowUser(userIdStr, profileUserIDStr); err != nil {
				errorData.status = http.StatusNotFound
				errorData.error_msg = "Failed to unfollow user"
				c.AbortWithStatusJSON(http.StatusNotFound, errorData)
				return
			}

			c.JSON(http.StatusNoContent, "")
		} else {
			errorData.status = http.StatusNotFound
			errorData.error_msg = "No 'follow' or 'unfollow' provided in request"
			c.AbortWithStatusJSON(http.StatusNotFound, errorData)
			return
		}
	}
}
