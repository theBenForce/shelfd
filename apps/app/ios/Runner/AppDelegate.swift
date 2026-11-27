import Flutter
import UIKit

@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate {
  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
    if let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "ShelfdWakeLock") {
      let channel = FlutterMethodChannel(
        name: "com.justwriteapps.shelfd/wakelock",
        binaryMessenger: registrar.messenger()
      )
      channel.setMethodCallHandler { call, result in
        if call.method == "setKeepScreenOn" {
          let args = call.arguments as? [String: Any]
          let enabled = args?["enabled"] as? Bool ?? false
          DispatchQueue.main.async {
            UIApplication.shared.isIdleTimerDisabled = enabled
            result(enabled)
          }
        } else {
          result(FlutterMethodNotImplemented)
        }
      }
    }
  }
}
