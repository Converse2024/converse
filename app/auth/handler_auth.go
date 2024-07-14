package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sourjaya/converse/app/templates/errorComponent"
	"github.com/Sourjaya/converse/app/templates/pages"
	v "github.com/Sourjaya/converse/app/validate"
	"github.com/Sourjaya/converse/env"
	"github.com/labstack/echo/v4"
)

var registerPage1Schema = v.Schema{
	"email": v.Rules(v.Email, v.CheckEmailDB("email", env.GetDBApiURI())),
}

var signupSchema = v.Schema{
	"email": v.Rules(v.Email, v.CheckEmailDB("email", env.GetDBApiURI())),
	"password": v.Rules(
		v.ContainsSpecial,
		v.ContainsUpper,
		v.Min(7),
		v.Max(50),
	),
	"firstName": v.Rules(v.Min(2), v.Max(50)),
	"lastName":  v.Rules(v.Min(2), v.Max(50)),
	"username":  v.Rules(v.Min(5), v.Max(15)),
}

var values pages.RegisterFormValues

func NotFoundHandler(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	components := errorComponent.Error404()
	return components.Render(c.Request().Context(), c.Response().Writer)
}

func handleGet(c echo.Context) error {

	t := c.QueryParam("check")
	switch t {
	case "email":
		var errors v.Errors
		errors, _ = v.Request(c.Request(), &values, registerPage1Schema)
		components := pages.EmailInput(values, errors)
		return components.Render(c.Request().Context(), c.Response().Writer)
	case "resendOTP":
		fmt.Println("RESENDING OTP")
		err := handleSendOTP(values.Email)
		if err != nil {
			components := errorComponent.Error500Component()
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
		components := pages.Resend()
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
				components := pages.RegisterForm(values, errors)
				return components.Render(c.Request().Context(), c.Response().Writer)
			}
			err := handleSendOTP(values.Email)
			components := pages.Otp(err)
			return components.Render(c.Request().Context(), c.Response().Writer)
		case "2":
			otp := c.FormValue("otp1") + c.FormValue("otp2") + c.FormValue("otp3") + c.FormValue("otp4")
			fmt.Println("OTP : ", otp)
			err := handleVerifyOTP(values.Email, otp)
			components := pages.Details(err)
			return components.Render(c.Request().Context(), c.Response().Writer)
		}
	} else {
		return handleGet(c)
	}
	return nil
}
