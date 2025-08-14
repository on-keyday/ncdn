use edge_api::api::{self, edge::DiffKind};

#[no_mangle]
pub extern "C" fn on_request() {
    let mut buffer = vec![0u8; 2048];
    let req = match api::get_request(&mut buffer) {
        Ok(req) => {
            req
        }
        Err(err) => {
            log::error!("[Acme] Failed to decode request info: {}", err);
            return;
        }
    };
    if !req.is_tls() {
        let host = req.host();
        let path = if req.path() == "/" {
            "/index.html"
        } else {
            req.path()
        };
        let location = format!("https://{}{}", host, path);
        let redirect_text = format!("Redirecting to {}", location);
        log::info!("[Acme] Redirecting to {}", location);
        api::ChangeSet::new()
            .request_routing(api::Routing::deny)
            .status(302)
            .location(&location)
            .response_body(&redirect_text.as_bytes())
            .apply().expect("Failed to create request changes");
    }
}

#[no_mangle]
pub extern "C" fn on_response() {
    api::ChangeSet::new().
        response_field(api::DiffKind::replace,"Alt-Svc","h3=\":443\"")
        .apply().expect("Failed to create response changes");
}