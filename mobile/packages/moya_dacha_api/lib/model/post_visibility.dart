//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

/// Кто видит пост (specs/013-post-visibility.md): `all` — все, а у закрытого профиля — подписчики; `friends` — те, с кем автор подписан друг на друга; `me` — только автор. 
enum PostVisibility {
  all._(r'all'),
  friends._(r'friends'),
  me._(r'me'),
  ;

  /// Instantiate a new enum with the provided value.
  const PostVisibility._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [PostVisibility] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static PostVisibility? fromJson(dynamic value) => PostVisibilityTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [PostVisibility]
  /// that were successfully decoded from the passed [JSON][json].
  static List<PostVisibility> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <PostVisibility>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = PostVisibility.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [PostVisibility] to String,
/// and [decode] dynamic data back to [PostVisibility].
class PostVisibilityTypeTransformer {
  factory PostVisibilityTypeTransformer() => _instance ??= const PostVisibilityTypeTransformer._();

  const PostVisibilityTypeTransformer._();

  /// Encodes this enum as a value suitable for JSON.
  String encode(PostVisibility data) => data._value;

  /// Returns the instance of [PostVisibility] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  PostVisibility? decode(dynamic data, {bool allowNull = true}) {
    if (data is PostVisibility) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'all': return PostVisibility.all;
        case r'friends': return PostVisibility.friends;
        case r'me': return PostVisibility.me;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static PostVisibilityTypeTransformer? _instance;
}

