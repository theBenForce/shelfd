package com.justwriteapps.shelfd

import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.view.WindowManager
import android.webkit.CookieManager
import com.google.android.gms.common.api.ApiException
import com.google.android.gms.common.api.CommonStatusCodes
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning
import com.ryanheise.audioservice.AudioServiceActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel

class MainActivity : AudioServiceActivity() {
    private var qrChannel: MethodChannel? = null
    private var wakeLockChannel: MethodChannel? = null
    private var pendingPairUri: String? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        capturePairIntent(intent, pushToDart = false)
        super.onCreate(savedInstanceState)
    }

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        val channel = MethodChannel(
            flutterEngine.dartExecutor.binaryMessenger,
            CHANNEL_NAME
        )
        qrChannel = channel
        channel.setMethodCallHandler(::handleMethodCall)

        val wlChannel = MethodChannel(
            flutterEngine.dartExecutor.binaryMessenger,
            WAKELOCK_CHANNEL_NAME
        )
        wakeLockChannel = wlChannel
        wlChannel.setMethodCallHandler(::handleWakeLockCall)
    }

    override fun cleanUpFlutterEngine(flutterEngine: FlutterEngine) {
        qrChannel?.setMethodCallHandler(null)
        qrChannel = null
        wakeLockChannel?.setMethodCallHandler(null)
        wakeLockChannel = null
        super.cleanUpFlutterEngine(flutterEngine)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        capturePairIntent(intent, pushToDart = true)
    }

    private fun capturePairIntent(intent: Intent?, pushToDart: Boolean) {
        val data = intent?.data ?: return
        if (!data.scheme.equals("shelfd", ignoreCase = true) ||
            !data.host.equals("pair", ignoreCase = true)
        ) {
            return
        }
        val rawUri = data.toString()
        intent.data = null
        val channel = qrChannel
        if (pushToDart && channel != null) {
            channel.invokeMethod("onPairUri", rawUri)
        } else {
            pendingPairUri = rawUri
        }
    }

    private fun handleMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            "scanQrCode" -> startQrScan(result)
            "getDeviceModel" -> {
                val manufacturer = Build.MANUFACTURER?.trim().orEmpty()
                val model = Build.MODEL?.trim().orEmpty()
                val combined =
                    if (model.startsWith(manufacturer, ignoreCase = true)) {
                        model
                    } else {
                        "$manufacturer $model".trim()
                    }
                result.success(combined.ifEmpty { "Android Device" })
            }
            "getInitialPairUri" -> {
                val uri = pendingPairUri
                pendingPairUri = null
                result.success(uri)
            }
            "setWebViewCookie" -> {
                val url = call.argument<String>("url")
                val cookie = call.argument<String>("cookie")
                if (url.isNullOrBlank() || cookie.isNullOrBlank()) {
                    result.error("INVALID_ARGS", "url and cookie are required", null)
                    return
                }
                val cookieManager = CookieManager.getInstance()
                cookieManager.setAcceptCookie(true)
                cookieManager.setCookie(url, cookie) {
                    cookieManager.flush()
                    result.success(true)
                }
            }
            "clearWebViewCookies" -> {
                val cookieManager = CookieManager.getInstance()
                cookieManager.removeAllCookies { removed ->
                    cookieManager.flush()
                    result.success(removed)
                }
            }
            else -> result.notImplemented()
        }
    }

    private fun startQrScan(result: MethodChannel.Result) {
        val options = GmsBarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .build()
        GmsBarcodeScanning.getClient(this, options)
            .startScan()
            .addOnSuccessListener { barcode ->
                result.success(barcode.rawValue)
            }
            .addOnCanceledListener {
                result.success(null)
            }
            .addOnFailureListener { e ->
                if (e is ApiException && e.statusCode == CommonStatusCodes.CANCELED) {
                    result.success(null)
                } else {
                    result.error("UNAVAILABLE", e.message ?: "QR scanner unavailable", null)
                }
            }
    }

    private fun handleWakeLockCall(call: MethodCall, result: MethodChannel.Result) {
        if (call.method == "setKeepScreenOn") {
            val enabled = call.argument<Boolean>("enabled") ?: false
            runOnUiThread {
                if (enabled) {
                    window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                } else {
                    window.clearFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
                }
                result.success(enabled)
            }
        } else {
            result.notImplemented()
        }
    }

    companion object {
        private const val CHANNEL_NAME = "com.justwriteapps.shelfd/qr_scanner"
        private const val WAKELOCK_CHANNEL_NAME = "com.justwriteapps.shelfd/wakelock"
    }
}
