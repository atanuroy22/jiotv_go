package cmd

import (
	"fmt"

	"github.com/jiotv-go/jiotv_go/v3/pkg/utils"
)

// Logout logs the user out by removing the saved login credentials file.
// It checks if the file exists before removing to avoid errors.
// Logs messages to provide feedback to the user.
// Returns any errors encountered.
func Logout() error {
	fmt.Println("Deleting existing login file if exists")

	err := utils.Logout()
	if err != nil {
		return err
	}

	fmt.Println("We have successfully logged you out. Please login again.")

	return nil
}

// LoginOTP handles the login flow using OTP.
// It takes the mobile number as input, sends an OTP,
// verifies the entered OTP by the user and logs in the user.
// Returns any error encountered.
func LoginOTP() error {
	fmt.Print("Enter your mobile number: +91 ")
	var mobileNumber string
	fmt.Scanln(&mobileNumber)
	mobileNumber = "+91" + mobileNumber

	fmt.Println("Sending OTP to your mobile number")

	result, err := utils.LoginSendOTP(mobileNumber)
	if err != nil {
		return err
	}

	if result {
		fmt.Println("OTP sent to your mobile number")

		fmt.Print("Enter OTP: ")
		var otp string
		fmt.Scanln(&otp)

		resultOTP, err := utils.LoginVerifyOTP(mobileNumber, otp)
		if err != nil {
			return err
		}

		if resultOTP["status"] == "success" {
			fmt.Println("Login successful")
		} else {
			fmt.Println("Login failed")
		}
	}

	return nil
}
