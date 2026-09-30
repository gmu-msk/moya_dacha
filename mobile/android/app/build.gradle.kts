import java.util.Properties

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

// Постоянный ключ подписи (ADR-0021). key.properties кладёт на CI
// deploy/signing-key.sh; без него сборка подписана debug-ключом машины,
// как любая локальная.
val keyProperties = Properties().apply {
    val file = rootProject.file("key.properties")
    if (file.exists()) file.inputStream().use { load(it) }
}
val hasReleaseKey = !keyProperties.isEmpty

// Настройки Firebase для пушей (specs/024-push.md, ADR-0027).
// google-services.json кладёт на CI mobile/tool/firebase-config.sh из
// секрета, в git его нет. Из него в ресурсы идут ровно те строки, по
// которым Firebase поднимается сам, — то же, что сделал бы плагин
// com.google.gms.google-services, но без него: без файла сборка просто
// идёт без пушей, а не падает.
@Suppress("UNCHECKED_CAST")
val firebaseValues: Map<String, String> = run {
    val file = file("google-services.json")
    if (!file.exists()) return@run emptyMap()
    val json = groovy.json.JsonSlurper().parse(file) as Map<String, Any?>
    val project = json["project_info"] as Map<String, Any?>
    val clients = json["client"] as List<Map<String, Any?>>
    val client = clients.firstOrNull {
        val info = it["client_info"] as Map<String, Any?>
        val android = info["android_client_info"] as Map<String, Any?>
        android["package_name"] == "ru.moyadacha.app"
    } ?: error("В google-services.json нет приложения ru.moyadacha.app")
    val info = client["client_info"] as Map<String, Any?>
    val keys = client["api_key"] as List<Map<String, Any?>>
    mapOf(
        "google_app_id" to info["mobilesdk_app_id"] as String,
        "gcm_defaultSenderId" to project["project_number"] as String,
        "project_id" to project["project_id"] as String,
        "google_api_key" to keys.first()["current_key"] as String,
    )
}

android {
    namespace = "ru.moyadacha.app"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        applicationId = "ru.moyadacha.app"
        // You can update the following values to match your application needs.
        // For more information, see: https://flutter.dev/to/review-gradle-config.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        // Uses the version code from pubspec.yaml. When using split APKs, 1000 * ABI_VERSION
        // is added automatically by Flutter. (https://developer.android.com/studio/build/configure-apk-splits#configure-APK-versions)
        // You can force using the value of versionCode by specifying the `-P force-version-code-ignoring-abi=true`
        // flag during build.
        versionCode = flutter.versionCode
        versionName = flutter.versionName
        firebaseValues.forEach { (name, value) -> resValue("string", name, value) }
    }

    // Строки Firebase выше идут через resValue, а в AGP 9 оно по
    // умолчанию выключено.
    buildFeatures {
        resValues = true
    }

    signingConfigs {
        if (hasReleaseKey) {
            create("release") {
                storeFile = file(keyProperties.getProperty("storeFile"))
                storePassword = keyProperties.getProperty("storePassword")
                keyAlias = keyProperties.getProperty("keyAlias")
                keyPassword = keyProperties.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.getByName(if (hasReleaseKey) "release" else "debug")
        }
        // Сборка из PR — debug, и ставится она поверх сборки из main:
        // подпись у них должна быть одна.
        debug {
            if (hasReleaseKey) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
