//go:build !js

package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/Sourjaya/converse/app/templates/errorComponent"
	"github.com/Sourjaya/converse/app/templates/pages"
	v "github.com/Sourjaya/converse/app/validate"
	"github.com/Sourjaya/converse/env"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
)

var registerPage1Schema = v.Schema{
	"email": v.Rules(v.Email, v.CheckEmailDB("email", env.GetDBApiURI())),
}

var registerPage3Schema = v.Schema{
	"firstName": v.Rules(v.Min(2), v.Max(50), v.Required),
	"lastName":  v.Rules(v.Min(2), v.Max(50), v.Required),
	"password": v.Rules(
		v.ContainsSpecial,
		v.ContainsUpper,
		v.Min(8),
		v.Max(50),
		v.Required,
	),
	"passwordConfirm": v.Rules(
		v.Required,
	),
}

var registerPage4Schema = v.Schema{
	"username": v.Rules(v.Min(5), v.Max(20), v.CheckUsernameDB("username", env.GetDBApiURI()), v.Required),
}

//var values pages.RegisterFormValues

func NotFoundHandler(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	components := errorComponent.Error404()
	return components.Render(c.Request().Context(), c.Response().Writer)
}
func Error500Handler(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Response().Header().Set("Retry-After", "3600")
	slog.Error("internal server error", "path", c.Request().URL.Path)
	components := errorComponent.Error500Component()

	return components.Render(c.Request().Context(), c.Response().Writer)
}
func handleValidity(c echo.Context) error {
	var values pages.RegisterFormValues
	uuid := c.QueryParam("id")
	sess, err := session.Get(fmt.Sprintf("reg_session_%v", uuid), c)
	if err != nil {
		fmt.Println("SOMETHING : ", err)
		return http.ErrNoCookie
	}
	t := c.QueryParam("check")
	switch t {
	case "email":
		var errors v.Errors
		var ok bool
		errors, ok = v.Request(c.Request(), &values, registerPage1Schema)
		if !ok {
			fmt.Println("NOT OKAY")
			components := pages.EmailInput(&values, errors)
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
		components := pages.EmailInput(&values, errors)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "resendOTP":
		fmt.Println("RESENDING OTP")
		values.Email = sess.Values["email"].(string)
		err := handleSendOTP(values.Email)
		if err != nil {
			Error500Handler(c)
		}
		components := pages.Resend(&values)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "details":
		var errors v.Errors
		var ok bool
		//fmt.Println("Details: ", values)
		errors, ok = v.Request(c.Request(), &values, registerPage3Schema)
		values.Uuid = uuid
		if !ok {
			components := pages.DetailsInput(&values, errors)
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
		components := pages.DetailsInput(&values, errors)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "password":
		var errors v.Errors

		toggleP := c.QueryParam("toggleP")
		toggleC := c.QueryParam("toggleC")
		toggle := pages.Toggle{Password: toggleP, ConfirmPass: toggleC}
		errors, _ = v.Request(c.Request(), &values, registerPage3Schema)

		if values.Password != values.PasswordConfirm {
			//fmt.Println("not matching")
			errors.Add("passwordConfirm", "passwords do not match")
		}
		components := pages.PasswordInput(&values, errors, toggle)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "username":
		var errors v.Errors
		errors, _ = v.Request(c.Request(), &values, registerPage4Schema)
		croppedImageData := c.FormValue("croppedImageData")
		if croppedImageData == "" {
			errors.Add("imageNotFound", "image not yet selected")
		}
		components := pages.Username(&values, errors)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "image":
		var errors v.Errors
		croppedImage := c.FormValue("croppedImageData")
		if croppedImage == "" {
			errors.Add("imageNotFound", "image not yet selected")
		}
		components := pages.Button(errors)
		return components.Render(c.Request().Context(), c.Response().Writer)
	}
	return nil
}

func handleVerifyOTP(email, otp string) error {
	var status error
	var otpResponse OtpApiResponse
	fmt.Printf("EMAIL : %v\nOTP : %v", email, otp)
	otpData := OtpVerifyRequest{
		Email: email,
		Otp:   otp,
	}

	// Marshal the struct to JSON
	jsonData, err := json.Marshal(otpData)
	if err != nil {
		return errors.New("error : Failed to Marshal data")
	}
	maxRetries := 3
	retryDelay := 5 * time.Second
	for i := 0; i < maxRetries; i++ {
		// Create a new HTTP request
		req, err := http.NewRequest(http.MethodPost, "http://localhost:5001/api/otp/verify", bytes.NewBuffer(jsonData))
		if err != nil {
			status = errors.New("error : Failed to Create Request")
			time.Sleep(retryDelay)
			continue
		}

		// Set the content type to application/json
		req.Header.Set("Content-Type", "application/json")

		// Create a new HTTP client and send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			status = errors.New("error : Failed to Send Request")
			time.Sleep(retryDelay)
			continue
		}
		defer resp.Body.Close()
		fmt.Println("Succesfull request send")
		// Check the response status code
		if resp.StatusCode != http.StatusOK {
			_ = json.NewDecoder(resp.Body).Decode(&otpResponse)
			if otpResponse.Error != "" {
				return fmt.Errorf("error : %v", otpResponse.Error)
			}
			status = fmt.Errorf("error : %v", resp.StatusCode)
			time.Sleep(retryDelay)
			continue
		}
		if status == nil {
			break
		}
	}
	fmt.Println(status)
	return status
}

func handleSendOTP(email string) error {
	var status error
	otpData := OtpGenerateRequest{
		Email:        email,
		Type:         "alphanumeric",
		Organization: "Converse",
		Subject:      "OTP Verification",
	}

	// Marshal the struct to JSON
	jsonData, err := json.Marshal(otpData)
	if err != nil {
		return errors.New("error : Failed to Marshal data")
	}
	maxRetries := 2
	retryDelay := 3 * time.Second
	for i := 0; i < maxRetries; i++ {
		// Create a new HTTP request
		req, err := http.NewRequest(http.MethodPost, "http://localhost:5001/api/otp/generate", bytes.NewBuffer(jsonData))
		if err != nil {
			status = errors.New("error : Failed to Create Request")
			time.Sleep(retryDelay)
			continue
		}

		// Set the content type to application/json
		req.Header.Set("Content-Type", "application/json")

		// Create a new HTTP client and send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			status = errors.New("error : Failed to Send Request")
			time.Sleep(retryDelay)
			continue
		}
		defer resp.Body.Close()
		fmt.Println("Succesfull request send")
		// Check the response status code
		if resp.StatusCode != http.StatusOK {
			status = fmt.Errorf("error : %v", resp.StatusCode)
			time.Sleep(retryDelay)
			continue
		}
		if status == nil {
			break
		}
	}
	return status
}
func HandleRegistration(c echo.Context) error {
	var values pages.RegisterFormValues
	page := c.QueryParam("page")
	if page != "" {
		switch page {
		case "1":
			errors, ok := v.Request(c.Request(), &values, registerPage1Schema)
			if !ok {
				components := pages.RegisterForm(&values, errors)
				return components.Render(c.Request().Context(), c.Response().Writer)
			}

			uuid := generateUUID()

			sess, err := session.Get(fmt.Sprintf("reg_session_%v", uuid), c)
			if err != nil {
				return http.ErrNoCookie
			}
			sess.Values["email"] = values.Email
			fmt.Printf("Email at case 1 : %v", sess.Values["email"].(string))
			sess.Save(c.Request(), c.Response())

			if err := handleSendOTP(values.Email); err != nil {
				return Error500Handler(c)
			}
			if c.Request().Header.Get("HX-Request") != "" {
				c.Response().Header().Set("HX-Redirect", fmt.Sprintf("/register?id=%v", uuid))
				return nil
			}

			// Normal HTTP redirect for non-HTMX requests
			return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/register?id=%v", uuid))
		case "2":
			otp := c.FormValue("otp1") + c.FormValue("otp2") + c.FormValue("otp3") + c.FormValue("otp4")

			uuid := c.QueryParam("id")
			sess, err := session.Get(fmt.Sprintf("reg_session_%v", uuid), c)
			if err != nil {
				return http.ErrNoCookie
			}
			fmt.Printf("Email at case 2 : %v", sess.Values["email"].(string))
			values.Email = sess.Values["email"].(string)
			values.Uuid = uuid
			err = handleVerifyOTP(values.Email, otp)
			if err != nil {
				components := errorComponent.Error500()
				return components.Render(c.Request().Context(), c.Response().Writer)
			} else {
				var errors v.Errors
				components := pages.DetailsForm(&values, errors, pages.Toggle{Password: "show", ConfirmPass: "show"})
				return components.Render(c.Request().Context(), c.Response().Writer)
			}
		case "3":
			uuid := c.QueryParam("id")
			sess, err := session.Get(fmt.Sprintf("reg_session_%v", uuid), c)
			errors, ok := v.Request(c.Request(), &values, registerPage3Schema)
			values.Uuid = uuid
			if !ok {
				components := pages.DetailsForm(&values, errors, pages.Toggle{Password: "show", ConfirmPass: "show"})
				return components.Render(c.Request().Context(), c.Response().Writer)
			}
			//errors = v.Errors{}

			if err != nil {
				return http.ErrNoCookie
			}
			sess.Values["firstName"] = values.FirstName
			sess.Values["lastName"] = values.LastName
			sess.Values["password"] = values.Password
			fmt.Printf("Session values : %v", sess.Values)
			sess.Save(c.Request(), c.Response())
			components := pages.Page4(&values, errors)
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
	} else {
		return handleValidity(c)
	}
	return nil
}
func HandleRedirectRegistration(c echo.Context) error {
	var values pages.RegisterFormValues
	values.Uuid = c.QueryParam("id")
	if v.IsValidUUID(values.Uuid) {
		components := pages.OtpPage(&values)
		return components.Render(c.Request().Context(), c.Response().Writer)
	} else {
		Error500Handler(c)
	}
	return nil
}
func HandleShowPassword(c echo.Context) error {
	var values pages.RegisterFormValues
	fmt.Println("CLICKED")
	toggleP := c.QueryParam("toggleP")
	toggleC := c.QueryParam("toggleC")
	toggle := pages.Toggle{Password: toggleP, ConfirmPass: toggleC}
	fmt.Println(toggle)
	v.Request(c.Request(), &values, registerPage3Schema)
	//fmt.Println(errors)
	components := pages.ViewPassword(&values, toggle)
	return components.Render(c.Request().Context(), c.Response().Writer)
}
func HandleShowConfirmPassword(c echo.Context) error {
	var values pages.RegisterFormValues
	fmt.Println("CLICKED")
	toggleP := c.QueryParam("toggleP")
	toggleC := c.QueryParam("toggleC")
	toggle := pages.Toggle{Password: toggleP, ConfirmPass: toggleC}
	fmt.Println(toggle)
	v.Request(c.Request(), &values, registerPage3Schema)
	//fmt.Println(errors)
	components := pages.ViewConfirmPassword(&values, toggle)
	return components.Render(c.Request().Context(), c.Response().Writer)
}

func HandleSignup(c echo.Context) error {
	var values pages.RegisterFormValues
	uuid := c.QueryParam("id")
	sess, err := session.Get(fmt.Sprintf("reg_session_%v", uuid), c)
	if err != nil {
		return http.ErrNoCookie
	}
	values.Email = sess.Values["email"].(string)
	values.FirstName = sess.Values["firstName"].(string)
	values.LastName = sess.Values["lastName"].(string)
	values.Password = sess.Values["password"].(string)
	registrationData := User{
		FirstName: values.FirstName,
		LastName:  values.LastName,
		Email:     values.Email,
		Password:  values.Password,
		IsAdmin:   false,
	}
	fmt.Println("------------------------------------")
	fmt.Println("")
	fmt.Println(values)
	fmt.Println("")
	fmt.Println("------------------------------------")
	username := c.FormValue("username")
	fmt.Println("\nUsername = ", username)
	base64ImageData := c.FormValue("croppedImageData")
	fmt.Println("\nBase64ImageData = ", base64ImageData)
	if base64ImageData == "" {
		fmt.Println("ERROR 1")
		return echo.NewHTTPError(http.StatusBadRequest, "No image data provided")
	}

	// Remove the base64 header if present (e.g., "data:image/jpeg;base64,")
	if strings.Contains(base64ImageData, ",") {
		parts := strings.Split(base64ImageData, ",")
		base64ImageData = parts[1]
	}

	// Decode the base64 string into a byte slice
	imageData, err := base64.StdEncoding.DecodeString(base64ImageData)
	if err != nil {
		fmt.Println("ERROR 2")
		return echo.NewHTTPError(http.StatusBadRequest, "Failed to decode base64 image data")
	}

	var requestBody bytes.Buffer
	multipartWriter := multipart.NewWriter(&requestBody)

	// Create a form file field and write the image data
	formFileWriter, err := multipartWriter.CreateFormFile("file", "cropped-image.jpg")
	if err != nil {
		fmt.Println("ERROR 3")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create form file field")
	}
	if _, err := io.Copy(formFileWriter, bytes.NewReader(imageData)); err != nil {
		fmt.Println("ERROR 4")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to write image data to form file field")
	}

	// Close the multipart writer to finalize the request body
	multipartWriter.Close()

	// Send the file to the target API server
	resp, err := http.Post("http://localhost:7070/api/file", multipartWriter.FormDataContentType(), &requestBody)
	if err != nil {
		fmt.Println("ERROR 5")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to send POST request to API server")
	}
	defer resp.Body.Close()

	// Read the response from the API server
	var responseBody bytes.Buffer
	if _, err := responseBody.ReadFrom(resp.Body); err != nil {
		fmt.Println("ERROR 6")
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read response from API server")
	}

	// Decode the JSON response
	var result CDNResponse
	decoder := json.NewDecoder(&responseBody)
	if err := decoder.Decode(&result); err != nil {
		fmt.Println("ERROR 7")
		return echo.NewHTTPError(http.StatusInternalServerError, "Error decoding JSON:", err)
	}

	// Extract the Cloudinary URL
	cloudinaryURL := result.Data.Data
	fmt.Println("Cloudinary URL:", cloudinaryURL)
	registrationData.Username = username
	registrationData.ProfilePic = cloudinaryURL
	fmt.Println("------------------------------------")
	fmt.Println("")
	fmt.Println(registrationData)
	fmt.Println("")
	fmt.Println("------------------------------------")

	return nil
}
