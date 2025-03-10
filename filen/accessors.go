package filen

func (api *Filen) GetAPIKey() string {
	return api.client.APIKey
}
