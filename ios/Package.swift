// swift-tools-version: 6.0
import PackageDescription

// RungKit is the half of the iOS app that does not draw anything: the models,
// the wire format and the API client. It is a package rather than app code so
// it can be built and tested on Linux, where SwiftUI does not exist — which is
// what lets the contract against the Go API be checked by a machine rather
// than by reading both sides and hoping.
let package = Package(
    name: "RungKit",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(name: "RungKit", targets: ["RungKit"])
    ],
    targets: [
        .target(name: "RungKit"),
        .testTarget(
            name: "RungKitTests",
            dependencies: ["RungKit"],
            resources: [.copy("Fixtures")]
        )
    ]
)
