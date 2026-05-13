import SwiftUI
import AVFoundation

/// Thin SwiftUI wrapper around AVCaptureSession for QR scanning.
/// Emits the first detected payload via the `onCode` closure, then stops.
struct QRScannerView: UIViewControllerRepresentable {
    let onCode: (String) -> Void

    func makeUIViewController(context: Context) -> ScannerVC {
        ScannerVC(onCode: onCode)
    }
    func updateUIViewController(_ vc: ScannerVC, context: Context) {}

    final class ScannerVC: UIViewController, AVCaptureMetadataOutputObjectsDelegate {
        private let session = AVCaptureSession()
        private var preview: AVCaptureVideoPreviewLayer!
        private let onCode: (String) -> Void
        private var emitted = false

        init(onCode: @escaping (String) -> Void) {
            self.onCode = onCode
            super.init(nibName: nil, bundle: nil)
        }
        required init?(coder: NSCoder) { fatalError() }

        override func viewDidLoad() {
            super.viewDidLoad()
            view.backgroundColor = .black
            configure()
        }
        override func viewWillAppear(_ animated: Bool) {
            super.viewWillAppear(animated)
            // AVCaptureSession is a class (reference type) and thread-safe — copy
            // the reference into the closure so we don't touch MainActor state
            // from the background queue (Swift 6 strict concurrency).
            let session = self.session
            if !session.isRunning {
                DispatchQueue.global(qos: .userInitiated).async {
                    session.startRunning()
                }
            }
        }
        override func viewWillDisappear(_ animated: Bool) {
            super.viewWillDisappear(animated)
            if session.isRunning { session.stopRunning() }
        }
        override func viewDidLayoutSubviews() {
            super.viewDidLayoutSubviews()
            preview?.frame = view.bounds
        }

        private func configure() {
            guard let device = AVCaptureDevice.default(for: .video),
                  let input = try? AVCaptureDeviceInput(device: device),
                  session.canAddInput(input) else { return }
            session.addInput(input)

            let output = AVCaptureMetadataOutput()
            guard session.canAddOutput(output) else { return }
            session.addOutput(output)
            output.metadataObjectTypes = [.qr]
            output.setMetadataObjectsDelegate(self, queue: .main)

            preview = AVCaptureVideoPreviewLayer(session: session)
            preview.videoGravity = .resizeAspectFill
            preview.frame = view.bounds
            view.layer.addSublayer(preview)
        }

        // Protocol method is nonisolated; we forced delivery on `.main` via
        // setMetadataObjectsDelegate(self, queue: .main) so MainActor access is
        // safe in practice — assumeIsolated reflects that to the type system.
        nonisolated func metadataOutput(_ output: AVCaptureMetadataOutput,
                                        didOutput metadataObjects: [AVMetadataObject],
                                        from connection: AVCaptureConnection) {
            guard let first = metadataObjects.first as? AVMetadataMachineReadableCodeObject,
                  let s = first.stringValue, !s.isEmpty else { return }
            MainActor.assumeIsolated {
                guard !emitted else { return }
                emitted = true
                onCode(s)
            }
        }
    }
}
