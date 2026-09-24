//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

library openapi.api;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:collection/collection.dart';
import 'package:http/http.dart';
import 'package:intl/intl.dart';
import 'package:meta/meta.dart';

part 'api_client.dart';
part 'api_helper.dart';
part 'api_exception.dart';
part 'auth/authentication.dart';
part 'auth/api_key_auth.dart';
part 'auth/oauth.dart';
part 'auth/http_basic_auth.dart';
part 'auth/http_bearer_auth.dart';

part 'api/auth_api.dart';
part 'api/follows_api.dart';
part 'api/operations_api.dart';
part 'api/posts_api.dart';
part 'api/profile_api.dart';
part 'api/users_api.dart';

part 'model/auth_code_accepted.dart';
part 'model/auth_code_request.dart';
part 'model/auth_code_too_soon.dart';
part 'model/author.dart';
part 'model/author_list.dart';
part 'model/comment.dart';
part 'model/comment_draft.dart';
part 'model/comments.dart';
part 'model/current_user.dart';
part 'model/error.dart';
part 'model/feed.dart';
part 'model/follow_list.dart';
part 'model/follow_user.dart';
part 'model/health.dart';
part 'model/media.dart';
part 'model/nickname_update.dart';
part 'model/post.dart';
part 'model/post_draft.dart';
part 'model/privacy_update.dart';
part 'model/profile_update.dart';
part 'model/relation.dart';
part 'model/report_draft.dart';
part 'model/session_created.dart';
part 'model/session_info.dart';
part 'model/session_request.dart';
part 'model/user_profile.dart';


/// An [ApiClient] instance that uses the default values obtained from
/// the OpenAPI specification file.
var defaultApiClient = ApiClient();

const _delimiters = {'csv': ',', 'ssv': ' ', 'tsv': '\t', 'pipes': '|'};
const _dateEpochMarker = 'epoch';
const _deepEquality = DeepCollectionEquality();
final _dateFormatter = DateFormat('yyyy-MM-dd');
final _regList = RegExp(r'^List<(.*)>$');
final _regSet = RegExp(r'^Set<(.*)>$');
final _regMap = RegExp(r'^Map<String,(.*)>$');

bool _isEpochMarker(String? pattern) => pattern == _dateEpochMarker || pattern == '/$_dateEpochMarker/';
