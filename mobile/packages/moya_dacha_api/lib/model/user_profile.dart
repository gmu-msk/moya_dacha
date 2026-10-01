//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class UserProfile {
  /// Returns a new [UserProfile] instance.
  UserProfile({
    required this.id,
    required this.nickname,
    required this.name,
    required this.about,
    this.avatarUrl,
    required this.createdAt,
    required this.posts,
    required this.followers,
    required this.following,
    required this.closed,
    required this.blocked,
    this.place,
    this.relation,
  });

  /// Идентификатор пользователя (UUID)
  String id;

  /// Никнейм — крупно на странице пользователя
  String nickname;

  /// Полное имя, может быть пустым; на странице — под никнеймом
  String name;

  /// Короткое «о себе», может быть пустым
  String about;

  /// Ссылка на аватар или `null`, если аватара нет. Может быть относительной — клиент достраивает её до адреса сервиса. 
  String? avatarUrl;

  /// Когда пользователь зарегистрировался
  DateTime createdAt;

  /// Сколько у пользователя постов сейчас
  int posts;

  /// Сколько у пользователя подписчиков; заявки не в счёт
  int followers;

  /// На сколько человек подписан пользователь; заявки не в счёт
  int following;

  /// Закрыт ли профиль (specs/012-follows.md)
  bool closed;

  /// Заблокировал ли смотрящий этого человека; в своём профиле `false` (specs/022-edit-block-delete.md) 
  bool blocked;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  Place? place;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  Relation? relation;

  @override
  bool operator ==(Object other) => identical(this, other) || other is UserProfile &&
    other.id == id &&
    other.nickname == nickname &&
    other.name == name &&
    other.about == about &&
    other.avatarUrl == avatarUrl &&
    other.createdAt == createdAt &&
    other.posts == posts &&
    other.followers == followers &&
    other.following == following &&
    other.closed == closed &&
    other.blocked == blocked &&
    other.place == place &&
    other.relation == relation;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (nickname.hashCode) +
    (name.hashCode) +
    (about.hashCode) +
    (avatarUrl == null ? 0 : avatarUrl!.hashCode) +
    (createdAt.hashCode) +
    (posts.hashCode) +
    (followers.hashCode) +
    (following.hashCode) +
    (closed.hashCode) +
    (blocked.hashCode) +
    (place == null ? 0 : place!.hashCode) +
    (relation == null ? 0 : relation!.hashCode);

  @override
  String toString() => 'UserProfile[id=$id, nickname=$nickname, name=$name, about=$about, avatarUrl=$avatarUrl, createdAt=$createdAt, posts=$posts, followers=$followers, following=$following, closed=$closed, blocked=$blocked, place=$place, relation=$relation]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'nickname'] = this.nickname;
      json[r'name'] = this.name;
      json[r'about'] = this.about;
    if (this.avatarUrl != null) {
      json[r'avatar_url'] = this.avatarUrl;
    } else {
      json[r'avatar_url'] = null;
    }
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
      json[r'posts'] = this.posts;
      json[r'followers'] = this.followers;
      json[r'following'] = this.following;
      json[r'closed'] = this.closed;
      json[r'blocked'] = this.blocked;
    if (this.place != null) {
      json[r'place'] = this.place;
    } else {
      json[r'place'] = null;
    }
    if (this.relation != null) {
      json[r'relation'] = this.relation;
    } else {
      json[r'relation'] = null;
    }
    return json;
  }

  /// Returns a new [UserProfile] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static UserProfile? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "UserProfile[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "UserProfile[id]" has a null value in JSON.');
        assert(json.containsKey(r'nickname'), 'Required key "UserProfile[nickname]" is missing from JSON.');
        assert(json[r'nickname'] != null, 'Required key "UserProfile[nickname]" has a null value in JSON.');
        assert(json.containsKey(r'name'), 'Required key "UserProfile[name]" is missing from JSON.');
        assert(json[r'name'] != null, 'Required key "UserProfile[name]" has a null value in JSON.');
        assert(json.containsKey(r'about'), 'Required key "UserProfile[about]" is missing from JSON.');
        assert(json[r'about'] != null, 'Required key "UserProfile[about]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "UserProfile[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "UserProfile[created_at]" has a null value in JSON.');
        assert(json.containsKey(r'posts'), 'Required key "UserProfile[posts]" is missing from JSON.');
        assert(json[r'posts'] != null, 'Required key "UserProfile[posts]" has a null value in JSON.');
        assert(json.containsKey(r'followers'), 'Required key "UserProfile[followers]" is missing from JSON.');
        assert(json[r'followers'] != null, 'Required key "UserProfile[followers]" has a null value in JSON.');
        assert(json.containsKey(r'following'), 'Required key "UserProfile[following]" is missing from JSON.');
        assert(json[r'following'] != null, 'Required key "UserProfile[following]" has a null value in JSON.');
        assert(json.containsKey(r'closed'), 'Required key "UserProfile[closed]" is missing from JSON.');
        assert(json[r'closed'] != null, 'Required key "UserProfile[closed]" has a null value in JSON.');
        assert(json.containsKey(r'blocked'), 'Required key "UserProfile[blocked]" is missing from JSON.');
        assert(json[r'blocked'] != null, 'Required key "UserProfile[blocked]" has a null value in JSON.');
        return true;
      }());

      return UserProfile(
        id: mapValueOfType<String>(json, r'id')!,
        nickname: mapValueOfType<String>(json, r'nickname')!,
        name: mapValueOfType<String>(json, r'name')!,
        about: mapValueOfType<String>(json, r'about')!,
        avatarUrl: mapValueOfType<String>(json, r'avatar_url'),
        createdAt: mapDateTime(json, r'created_at', r'')!,
        posts: mapValueOfType<int>(json, r'posts')!,
        followers: mapValueOfType<int>(json, r'followers')!,
        following: mapValueOfType<int>(json, r'following')!,
        closed: mapValueOfType<bool>(json, r'closed')!,
        blocked: mapValueOfType<bool>(json, r'blocked')!,
        place: Place.fromJson(json[r'place']),
        relation: Relation.fromJson(json[r'relation']),
      );
    }
    return null;
  }

  static List<UserProfile> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <UserProfile>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = UserProfile.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, UserProfile> mapFromJson(dynamic json) {
    final map = <String, UserProfile>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = UserProfile.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of UserProfile-objects as value to a dart map
  static Map<String, List<UserProfile>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<UserProfile>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = UserProfile.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'nickname',
    'name',
    'about',
    'created_at',
    'posts',
    'followers',
    'following',
    'closed',
    'blocked',
  };
}

