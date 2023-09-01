package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"bytes"
	"github.com/gorilla/mux"
	"strconv"
)

// Define a struct for the LNPay API response
type LNPayResponse struct {
    ID             string `json:"id"`
    PaymentRequest string `json:"payment_request"`
}

func getInvoiceHandler(w http.ResponseWriter, r *http.Request) {
    // Define the request payload
    payload := map[string]interface{}{
        "num_satoshis": 777,
        "memo": "Access to satdress form",
    }

    jsonData, err := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.lnpay.co/v1/wallet/waki_pB9oopePeIi29JEbL3h9mU7k/invoice?fields=id,payment_request", bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Api-Key", "pak_Dkjy3ttZKOsquanaLNoj3VDJ5frS9M7")

	client := &http.Client{}
    resp, err := client.Do(req)

    if err != nil {
        http.Error(w, "Failed to call LNPay API", http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()
    // jsonData, err := json.Marshal(payload)
    // if err != nil {
    //     http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
    //     return
    // }

    // Make the HTTP POST request to LNPay API
    // req, err := http.NewRequest("POST", "https://api.lnpay.co/v1/wallet/waki_ucUzLVZgePHqOGbEchxAa6E/invoice?fields=id,payment_request", bytes.NewBuffer(jsonData))
    // req.Header.Set("Content-Type", "application/json")
    // req.Header.Set("X-Api-Key", "pak_g9017h233CD61lmB2PHaEGqBp1uSwh0")

    // client := &http.Client{}
    // resp, err := client.Do(req)
    // if err != nil {
    //     http.Error(w, "Failed to call LNPay API", http.StatusInternalServerError)
    //     return
    // }
    // defer resp.Body.Close()

    var lnPayResponse LNPayResponse
    err = json.NewDecoder(resp.Body).Decode(&lnPayResponse)
    if err != nil {
        http.Error(w, "Failed to decode LNPay API response", http.StatusInternalServerError)
        return
    }

    // Return the invoice details to the frontend
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{
        "id": "` + lnPayResponse.ID + `",
        "payment_request": "` + lnPayResponse.PaymentRequest + `"
    }`))
}

// Define a struct for the LNPay API response when checking invoice status
type LNPayInvoiceStatus struct {
    Settled int `json:"settled"`
}

func checkInvoiceHandler(w http.ResponseWriter, r *http.Request) {
    invoiceID := r.URL.Query().Get("id")
    if invoiceID == "" {
        http.Error(w, "Invoice ID is required", http.StatusBadRequest)
        return
    }

    // Make the HTTP GET request to LNPay API to check the invoice status
    req, err := http.NewRequest("GET", "https://api.lnpay.co/v1/lntx/"+invoiceID, nil)
    req.Header.Set("X-Api-Key", "pak_g9017h233CD61lmB2PHaEGqBp1uSwh0")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        http.Error(w, "Failed to call LNPay API", http.StatusInternalServerError)
        return
    }
    defer resp.Body.Close()

    var lnPayInvoiceStatus LNPayInvoiceStatus
    err = json.NewDecoder(resp.Body).Decode(&lnPayInvoiceStatus)
    if err != nil {
        http.Error(w, "Failed to decode LNPay API response", http.StatusInternalServerError)
        return
    }

    // Return the invoice status to the frontend
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{
        "settled": ` + strconv.Itoa(lnPayInvoiceStatus.Settled) + `
    }`))
}





type Response struct {
	Ok      bool        `json:"ok"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type SuccessClaim struct {
	Name    string `json:"name"`
	Domain  string `json:"domain"`
	PIN     string `json:"pin"`
	Invoice string `json:"invoice"`
}

// not authenticated, if correct pin is provided call returns the SuccessClaim
func ClaimAddress(w http.ResponseWriter, r *http.Request) {
	params := parseParams(r)
	pin, inv, err := SaveName(params.Name, params.Domain, params, params.Pin)
	if err != nil {
		sendError(w, 400, "could not register name: %s", err.Error())
		return
	}

	response := Response{
		Ok:      true,
		Message: fmt.Sprintf("claimed %v@%v", params.Name, params.Domain),
		Data:    SuccessClaim{params.Name, params.Domain, pin, inv},
	}

	// TODO: middleware for responses that adds this header
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	domain := mux.Vars(r)["domain"]
	params, err := GetName(name, domain)
	if err != nil {
		sendError(w, 400, err.Error())
		return
	}

	// add pin to response because sometimes not saved in database; after first call to /api/v1/claim
	params.Pin = ComputePIN(name, domain)

	response := Response{
		Ok:      true,
		Message: fmt.Sprintf("%v@%v found", params.Name, domain),
		Data:    params,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	params := parseParams(r)
	name := mux.Vars(r)["name"]
	domain := mux.Vars(r)["domain"]

	// if pin not in json request body get it from header
	if params.Pin == "" {
		// TODO: work with Context()?
		params.Pin = r.Header.Get("X-Pin")
	}

	if _, _, err := SaveName(name, domain, params, params.Pin); err != nil {
		sendError(w, 500, err.Error())
		return
	}

	updatedParams, err := GetName(name, domain)
	if err != nil {
		sendError(w, 500, err.Error())
		return
	}

	// return the updated values or just http.StatusCreated?
	response := Response{
		Ok:      true,
		Message: fmt.Sprintf("updated %v@%v parameters", params.Name, domain),
		Data:    updatedParams,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	domain := mux.Vars(r)["domain"]
	if err := DeleteName(name, domain); err != nil {
		sendError(w, 500, err.Error())
		return
	}

	response := Response{
		Ok:      true,
		Message: fmt.Sprintf("deleted %v@%v", name, domain),
		Data:    nil,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// authentication middleware
func authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check domain
		domain := mux.Vars(r)["domain"]
		available := getDomains(s.Domain)
		found := false
		for _, one := range available {
			if one == domain {
				found = true
			}
		}
		if !found {
			sendError(w, 400, "could not use domain: %s", domain)
			return
		}

		// exempt /claim from authentication check;
		if strings.HasPrefix(r.URL.Path, "/api/v1/claim") {
			next.ServeHTTP(w, r)
			return
		}

		name := mux.Vars(r)["name"]
		providedPin := r.Header.Get("X-Pin")

		var err error

		if providedPin == "" {
			err = fmt.Errorf("X-Pin header not provided")
			// pin should always be passed in header but search in json request body anyways
			providedPin = parseParams(r).Pin
		}

		if providedPin != ComputePIN(name, domain) {
			err = fmt.Errorf("wrong pin")
		}

		if err != nil {
			sendError(w, 401, "error fetching user: %s", err.Error())
			return
		}

		next.ServeHTTP(w, r)
	})
}

// helpers
func sendError(w http.ResponseWriter, code int, msg string, args ...interface{}) {
	b, _ := json.Marshal(Response{false, fmt.Sprintf(msg, args...), nil})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(b)
}

func parseParams(r *http.Request) *Params {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var params Params
	json.Unmarshal(reqBody, &params)
	return &params
}
