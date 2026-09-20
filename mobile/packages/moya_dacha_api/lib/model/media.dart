//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Media {
  /// Returns a new [Media] instance.
  Media({
    required this.id,
    required this.kind,
    required this.url,
    required this.width,
    required this.height,
  });

  /// Идентификатор медиа (UUID)
  String id;

  /// Вид медиа. В MVP всегда `photo`.
  MediaKindEnum kind;

  /// Ссылка на файл. Может быть относительной — клиент достраивает её до адреса сервиса. 
  String url;

  /// Ширина в пикселях — чтобы занять место до загрузки
  int width;

  /// Высота в пикселях
  int height;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Media &&
    other.id == id &&
    other.kind == kind &&
    other.url == url &&
    other.width == width &&
    other.height == height;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (kind.hashCode) +
    (url.hashCode) +
    (width.hashCode) +
    (height.hashCode);

  @override
  String toString() => 'Media[id=$id, kind=$kind, url=$url, width=$width, height=$height]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'kind'] = this.kind;
      json[r'url'] = this.url;
      json[r'width'] = this.width;
      json[r'height'] = this.height;
    return json;
  }

  /// Returns a new [Media] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Media? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "Media[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "Media[id]" has a null value in JSON.');
        assert(json.containsKey(r'kind'), 'Required key "Media[kind]" is missing from JSON.');
        assert(json[r'kind'] != null, 'Required key "Media[kind]" has a null value in JSON.');
        assert(json.containsKey(r'url'), 'Required key "Media[url]" is missing from JSON.');
        assert(json[r'url'] != null, 'Required key "Media[url]" has a null value in JSON.');
        assert(json.containsKey(r'width'), 'Required key "Media[width]" is missing from JSON.');
        assert(json[r'width'] != null, 'Required key "Media[width]" has a null value in JSON.');
        assert(json.containsKey(r'height'), 'Required key "Media[height]" is missing from JSON.');
        assert(json[r'height'] != null, 'Required key "Media[height]" has a null value in JSON.');
        return true;
      }());

      return Media(
        id: mapValueOfType<String>(json, r'id')!,
        kind: MediaKindEnum.fromJson(json[r'kind'])!,
        url: mapValueOfType<String>(json, r'url')!,
        width: mapValueOfType<int>(json, r'width')!,
        height: mapValueOfType<int>(json, r'height')!,
      );
    }
    return null;
  }

  static List<Media> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Media>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Media.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Media> mapFromJson(dynamic json) {
    final map = <String, Media>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Media.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Media-objects as value to a dart map
  static Map<String, List<Media>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Media>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Media.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'kind',
    'url',
    'width',
    'height',
  };
}

/// Вид медиа. В MVP всегда `photo`.
enum MediaKindEnum {
  photo._(r'photo'),
  ;

  /// Instantiate a new enum with the provided value.
  const MediaKindEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [MediaKindEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static MediaKindEnum? fromJson(dynamic value) => MediaKindEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [MediaKindEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<MediaKindEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <MediaKindEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = MediaKindEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [MediaKindEnum] to String,
/// and [decode] dynamic data back to [MediaKindEnum].
class MediaKindEnumTypeTransformer {
  factory MediaKindEnumTypeTransformer() => _instance ??= const MediaKindEnumTypeTransformer._();

  const MediaKindEnumTypeTransformer._();

  String encode(MediaKindEnum data) => data._value;

  /// Returns the instance of [MediaKindEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  MediaKindEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is MediaKindEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'photo': return MediaKindEnum.photo;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static MediaKindEnumTypeTransformer? _instance;
}


