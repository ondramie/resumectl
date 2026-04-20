import UIKit
import UniformTypeIdentifiers

class ShareViewController: UIViewController, UIDocumentPickerDelegate {
    private let apiURL = "https://resumectl.fly.dev/match"

    private var apiToken: String {
        KeychainHelper.get(key: "api_token") ?? ""
    }

    private let statusLabel = UILabel()
    private let activityIndicator = UIActivityIndicatorView(style: .large)

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground

        activityIndicator.translatesAutoresizingMaskIntoConstraints = false
        activityIndicator.startAnimating()
        view.addSubview(activityIndicator)

        statusLabel.translatesAutoresizingMaskIntoConstraints = false
        statusLabel.text = "Matching resume..."
        statusLabel.textAlignment = .center
        statusLabel.font = .systemFont(ofSize: 18, weight: .medium)
        view.addSubview(statusLabel)

        NSLayoutConstraint.activate([
            activityIndicator.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            activityIndicator.centerYAnchor.constraint(equalTo: view.centerYAnchor, constant: -20),
            statusLabel.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            statusLabel.topAnchor.constraint(equalTo: activityIndicator.bottomAnchor, constant: 16),
        ])

        navigationItem.rightBarButtonItem = UIBarButtonItem(
            barButtonSystemItem: .cancel, target: self, action: #selector(cancel))

        extractURL { [weak self] url in
            guard let self = self, let url = url else {
                self?.showError("No URL found")
                return
            }
            self.runMatch(jobURL: url)
        }
    }

    func extractURL(completion: @escaping (String?) -> Void) {
        guard let items = extensionContext?.inputItems as? [NSExtensionItem] else {
            completion(nil)
            return
        }

        for item in items {
            guard let attachments = item.attachments else { continue }
            for provider in attachments {
                if provider.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
                    provider.loadItem(forTypeIdentifier: UTType.url.identifier) { item, _ in
                        if let url = item as? URL {
                            completion(url.absoluteString)
                        } else {
                            completion(nil)
                        }
                    }
                    return
                }
                if provider.hasItemConformingToTypeIdentifier(UTType.plainText.identifier) {
                    provider.loadItem(forTypeIdentifier: UTType.plainText.identifier) { item, _ in
                        if let text = item as? String, text.hasPrefix("http") {
                            completion(text)
                        } else {
                            completion(nil)
                        }
                    }
                    return
                }
            }
        }
        completion(nil)
    }

    func runMatch(jobURL: String) {
        var request = URLRequest(url: URL(string: apiURL)!)
        request.httpMethod = "POST"
        request.setValue("Bearer \(apiToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: ["url": jobURL])

        URLSession.shared.dataTask(with: request) { [weak self] data, _, error in
            DispatchQueue.main.async {
                guard let self = self else { return }

                if let error = error {
                    self.showError(error.localizedDescription)
                    return
                }

                guard let data = data,
                      let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
                    self.showError("Invalid response")
                    return
                }

                if let err = json["error"] as? String {
                    self.showError(err)
                    return
                }

                let score = json["score"] as? Int ?? 0
                let company = json["company"] as? String ?? ""
                let title = json["title"] as? String ?? ""

                self.statusLabel.text = "Score: \(score)/100\n\(title)\n\(company)\n\nSaving PDF..."
                self.statusLabel.numberOfLines = 0

                if let pdfPath = json["pdf_url"] as? String, !pdfPath.isEmpty {
                    self.downloadAndSharePDF(path: pdfPath)
                } else {
                    self.activityIndicator.stopAnimating()
                }
            }
        }.resume()
    }

    func downloadAndSharePDF(path: String) {
        let fullURL = "https://resumectl.fly.dev\(path)"
        var request = URLRequest(url: URL(string: fullURL)!)
        request.setValue("Bearer \(apiToken)", forHTTPHeaderField: "Authorization")

        URLSession.shared.dataTask(with: request) { [weak self] data, _, _ in
            guard let data = data else { return }

            let tmpURL = FileManager.default.temporaryDirectory.appendingPathComponent("resume.pdf")
            try? data.write(to: tmpURL)

            DispatchQueue.main.async {
                guard let self = self else { return }
                let picker = UIDocumentPickerViewController(forExporting: [tmpURL], asCopy: true)
                picker.delegate = self
                self.present(picker, animated: true)
            }
        }.resume()
    }

    func showError(_ message: String) {
        activityIndicator.stopAnimating()
        statusLabel.text = "Error: \(message)"
        statusLabel.textColor = .systemRed
    }

    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        extensionContext?.completeRequest(returningItems: nil)
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        extensionContext?.completeRequest(returningItems: nil)
    }

    @objc func cancel() {
        extensionContext?.completeRequest(returningItems: nil)
    }
}
