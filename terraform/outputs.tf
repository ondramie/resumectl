output "project_id" {
  value = google_project.resumectl.project_id
}

output "credentials_json" {
  sensitive = true
  value = jsonencode({
    installed = {
      client_id     = google_iap_client.resumectl.client_id
      client_secret = google_iap_client.resumectl.secret
      redirect_uris = ["urn:ietf:wg:oauth:2.0:oob", "http://localhost"]
      auth_uri      = "https://accounts.google.com/o/oauth2/auth"
      token_uri     = "https://oauth2.googleapis.com/token"
    }
  })
}
