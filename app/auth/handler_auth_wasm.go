//go:build js

package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"syscall/js"
	"time"

	"github.com/Sourjaya/converse/app/templates/errorComponent"
	"github.com/Sourjaya/converse/app/templates/pages"
	v "github.com/Sourjaya/converse/app/validate"
	"github.com/labstack/echo/v4"
)

var registerPage1Schema = v.Schema{
	"email": v.Rules(v.Email, v.CheckEmailDB("email", js.Global().Get("DB_API_URI").String())),
}

var registerPage3Schema = v.Schema{
	"firstName": v.Rules(v.Min(2), v.Max(50)),
	"lastName":  v.Rules(v.Min(2), v.Max(50)),
	"password": v.Rules(
		v.ContainsSpecial,
		v.ContainsUpper,
		v.Min(8),
		v.Max(50),
	),
}

var registerPage4Schema = v.Schema{
	"username": v.Rules(v.Min(5), v.Max(20), v.CheckUsernameDB("username", js.Global().Get("DB_API_URI").String())),
}

var values pages.RegisterFormValues

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
func handleGet(c echo.Context) error {

	t := c.QueryParam("check")
	switch t {
	case "email":
		var errors v.Errors
		var ok bool
		errors, ok = v.Request(c.Request(), &values, registerPage1Schema)
		if !ok {
			components := pages.EmailInput(&values, errors)
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
		components := pages.EmailInput(&values, errors)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "resendOTP":
		fmt.Println("RESENDING OTP")
		err := handleSendOTP(values.Email)
		if err != nil {
			Error500Handler(c)
		}
		components := pages.Resend()
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "details":
		var errors v.Errors
		var ok bool
		//fmt.Println("Details: ", values)
		email := values.Email
		errors, ok = v.Request(c.Request(), &values, registerPage3Schema)
		values.Email = email
		if !ok {
			components := pages.DetailsInput(&values, errors)
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
		components := pages.DetailsInput(&values, errors)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "password":
		var errors v.Errors
		email := values.Email
		toggleP := c.QueryParam("toggleP")
		toggleC := c.QueryParam("toggleC")
		toggle := pages.Toggle{Password: toggleP, ConfirmPass: toggleC}
		errors, _ = v.Request(c.Request(), &values, registerPage3Schema)
		values.Email = email
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
	fmt.Println(status)
	return status
}
func HandleRegistration(c echo.Context) error {
	page := c.QueryParam("page")
	if page != "" {
		switch page {
		case "1":
			errors, ok := v.Request(c.Request(), &values, registerPage1Schema)
			if !ok {
				components := pages.RegisterForm(&values, errors)
				return components.Render(c.Request().Context(), c.Response().Writer)
			}
			err := handleSendOTP(values.Email)
			components := pages.Otp(err)
			return components.Render(c.Request().Context(), c.Response().Writer)
		case "2":
			otp := c.FormValue("otp1") + c.FormValue("otp2") + c.FormValue("otp3") + c.FormValue("otp4")
			err := handleVerifyOTP(values.Email, otp)
			if err != nil {
				components := errorComponent.Error500()
				return components.Render(c.Request().Context(), c.Response().Writer)
			} else {
				var errors v.Errors
				components := pages.DetailsForm(&values, errors, pages.Toggle{Password: "show", ConfirmPass: "show"})
				return components.Render(c.Request().Context(), c.Response().Writer)
			}
		case "3":
			errors := v.Errors{}
			components := pages.Page4(&values, errors)
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
	} else {
		return handleGet(c)
	}
	return nil
}

func HandleShowPassword(c echo.Context) error {
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
	registrationData := User{
		FirstName:  values.FirstName,
		LastName:   values.LastName,
		Email:      values.Email,
		ProfilePic: values.ProfilePic,
		Username:   values.Username,
		Password:   values.Password,
		IsAdmin:    false,
	}
	_ = registrationData
	file, err := c.FormFile("croppedImageData")
	if err != nil {
		log.Println("Error: ", err)
		return err
	}
	_ = file
	return nil
}

func Upload(c echo.Context) error {
	file, err := c.FormFile("croppedImage")
	if err != nil {
		log.Println("Error: ", err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	fmt.Println("__________________Image found_________________________")
	_ = file
	return nil
}
